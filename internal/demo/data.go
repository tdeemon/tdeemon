package demo

import (
	"context"
	"fmt"
	"math"
	"time"
	"uuid"

	"github.com/tdeemon/tdeemon/internal/domain/calibration"
	"github.com/tdeemon/tdeemon/internal/domain/day"
	"github.com/tdeemon/tdeemon/internal/domain/food"
	"github.com/tdeemon/tdeemon/internal/domain/meal"
	"github.com/tdeemon/tdeemon/internal/domain/metric"
	"github.com/tdeemon/tdeemon/internal/domain/plan"
	"github.com/tdeemon/tdeemon/internal/domain/recipe"
	"github.com/tdeemon/tdeemon/internal/store"
)

// The body of the demo user, and how it moves.
const (
	kcalPerKg = plan.KcalPerKg
	// meanSteps is the steps of an average day; each step past it burns
	// stepKcalEach.
	meanSteps    = 9000
	stepKcalEach = 0.045
	// trainBonus is what a training day burns over the average, and
	// restBonus a rest day's, so a week of four of one and three of the
	// other averages to nothing.
	trainBonus = 90
	restBonus  = -120
	// glycogenKg is the water and glycogen the first days of a cut take
	// off the scale, glycogenDays how fast.
	glycogenKg   = 0.9
	glycogenDays = 3
	// waterCarry is how much of a day's water the next morning keeps,
	// waterSD the spread of a day's new water, and saltKg the water a
	// kcal eaten past the target holds the next morning.
	waterCarry = 0.5
	waterSD    = 0.25
	saltKg     = 0.0012
	// fatShare is the share of the mass lost that is fat.
	fatShare = 0.85
	// startBodyFat is the body fat the cut starts at, in %.
	startBodyFat = 22.5
)

// The chances of the days' gaps.
const (
	noWeighIn  = 0.12
	noDevice   = 0.04
	notLogged  = 0.03
	checkedIn  = 0.55
	noteChance = 0.12
)

// body is the demo user's body as the days pass: the weight the energy
// balance leaves, the water on top of it, and the fat in it.
type body struct {
	kg, water, salt, fatKg float64
}

// The meals of a day, in order, with the share of the day's intake each
// has and when it is eaten: from its hour and minute, within spread
// minutes.
type slot struct {
	templates    [][]part
	share        float64
	chance       float64
	hour, minute int
	spread       int
	weekendLater bool
}

var slots = []slot{
	{templates: breakfasts, share: 0.25, chance: 0.95, hour: 7, spread: 90, weekendLater: true},
	{templates: lunches, share: 0.32, chance: 1, hour: 12, minute: 15, spread: 90},
	{templates: snacks, share: 0.10, chance: 0.7, hour: 16, spread: 60},
	{templates: dinners, share: 0.33, chance: 1, hour: 19, spread: 90},
	{templates: snacks, share: 0.08, chance: 0.35, hour: 21, minute: 15, spread: 60},
}

// checkInNotes are notes a day's check-in may have.
var checkInNotes = []name{
	{"Legs day, felt strong.", "Dan za noge, osjećao sam se snažno."},
	{"Slept badly, hungry all afternoon.", "Loše sam spavao, gladan cijelo popodne."},
	{"Long walk with the dog.", "Duga šetnja sa psom."},
	{"Busy at work, ate late.", "Gužva na poslu, jeo sam kasno."},
	{"New best on the bench press.", "Novi osobni rekord na benču."},
}

// The day out of calibration: a dinner at a restaurant, its plate an
// estimate and more eaten than it says.
var (
	restaurant       = name{"Restaurant dinner: burger and fries", "Večera u restoranu: burger i pomfrit"}
	restaurantReason = name{"Restaurant dinner, not weighed", "Večera u restoranu, nije vagano"}
	restaurantPlate  = food.Nutrients{Kcal: 1150, Protein: 48, Carbs: 95, Fat: 58}
)

// restaurantExtra is what the user ate at the restaurant past the
// estimate they logged.
const restaurantExtra = 600

// saveFoods stores the demo user's foods.
func (sd *seeder) saveFoods(ctx context.Context) error {
	now := sd.at(sd.start, 8, 5)
	for _, df := range foods {
		f := food.Food{
			ID: uuid.NewV7(), Name: df.name.in(sd.u.Locale), Basis: df.basis, Source: food.Reference,
			Nutrients: food.Nutrients{Kcal: df.kcal, Protein: df.p, Carbs: df.c, Fat: df.f},
		}
		if df.fiber > 0 {
			f.Nutrients.Fiber = new(df.fiber)
		}
		if df.serving != nil {
			f.Servings = []food.Serving{{Name: df.serving.in(sd.u.Locale), Amount: df.servingAmount}}
		}
		stored, err := sd.db.CreateFood(ctx, sd.u.ID, importActor, f, now)
		if err != nil {
			return fmt.Errorf("food %s: %w", f.Name, err)
		}
		sd.foods[df.key] = stored
		sd.res.Foods++
	}
	return nil
}

// saveRecipes stores the demo user's recipes, cooked on the first day.
func (sd *seeder) saveRecipes(ctx context.Context) error {
	now := sd.at(sd.start, 8, 6)
	for _, dr := range recipes {
		r := recipe.Recipe{
			Food:  food.Food{ID: uuid.NewV7(), Name: dr.name.in(sd.u.Locale)},
			Yield: dr.yield, Portions: dr.portions,
		}
		for _, in := range dr.ingredients {
			it, err := meal.FromFood(sd.foods[in.food], meal.Quantity{Amount: in.grams, Unit: "g"}, "")
			if err != nil {
				return fmt.Errorf("recipe %s: %w", r.Food.Name, err)
			}
			r.Items = append(r.Items, it)
		}
		stored, err := sd.db.CreateRecipe(ctx, sd.u.ID, importActor, r, nil, now)
		if err != nil {
			return fmt.Errorf("recipe %s: %w", r.Food.Name, err)
		}
		sd.foods[dr.key] = stored.Food
		sd.res.Recipes++
	}
	return nil
}

// history writes each day's data from the first to today, the weight
// following what the user ate against what they spent.
func (sd *seeder) history(ctx context.Context) error {
	b := body{kg: sd.w0, fatKg: sd.w0 * startBodyFat / 100}
	excluded := sd.start.AddDays(sd.o.Days * 3 / 5)
	var spent float64
	for d := sd.start; !sd.today.Before(d); d = d.AddDays(1) {
		out, err := sd.day(ctx, d, &b, d == excluded)
		if err != nil {
			return fmt.Errorf("day %s: %w", d, err)
		}
		spent += out
	}
	sd.res.TDEE = spent / float64(sd.o.Days)
	return nil
}

// day writes day d's data, its weight from b, and moves b by the day's
// energy balance. It returns the energy the day spent, 0 for today, which
// is not over.
func (sd *seeder) day(ctx context.Context, d day.Date, b *body, excluded bool) (float64, error) {
	k := d.DaysSince(sd.start)
	dt := plan.TypeOf(d, trainingDays(), false, nil)
	t := sd.target(sd.start).For(dt)
	cd, steps, err := sd.morning(ctx, d, b, k)
	if err != nil {
		return 0, err
	}
	if excluded {
		cd.Status = day.Excluded
	}

	// The day's meals, near the target, unless the user forgot to log
	// them.
	want := sd.appetite(d, t.Kcal)
	eaten := want
	if forgot := k > 0 && d != sd.today && !excluded && sd.chance(notLogged); !forgot {
		if cd.Kcal, cd.Meals, err = sd.meals(ctx, d, want, excluded); err != nil {
			return 0, err
		}
		eaten = cd.Kcal
		if excluded {
			eaten += restaurantExtra
		}
	}
	sd.days[d] = cd
	if d == sd.today {
		return 0, nil
	}
	if err := sd.checkIn(ctx, d, dt, excluded); err != nil {
		return 0, err
	}

	// The night: the balance of the day moves the weight.
	bonus := float64(restBonus)
	if dt == plan.Training {
		bonus = trainBonus
	}
	spent := sd.base + stepKcalEach*(steps-meanSteps) + bonus
	lost := (spent - eaten) / kcalPerKg
	b.kg -= lost
	b.fatKg -= fatShare * lost
	b.salt = 0.5*b.salt + saltKg*math.Max(0, eaten-t.Kcal-150)
	return spent, nil
}

// morning writes day d's measurements, the k-th day of the cut, the
// scale showing b's weight with its water, and returns the day as
// calibration reads it so far, and the steps the day will have.
func (sd *seeder) morning(ctx context.Context, d day.Date, b *body, k int) (calibration.Day, float64, error) {
	b.water = waterCarry*b.water + sd.normal(0, waterSD)
	shown := b.kg - glycogenKg*(1-math.Exp(-float64(k)/glycogenDays)) + b.water + b.salt
	at := sd.at(d, 7, 0).Add(time.Duration(sd.rng.IntN(60)) * time.Minute)
	cd := calibration.Day{Status: day.Complete}
	if d == sd.today {
		cd.Status = day.Open
		at = sd.morningOfToday(at)
	}
	// Today always has its morning, so the day a demo user first sees,
	// and the screenshots, show a weigh-in and the device's data; the
	// chances are drawn all the same, so the days before stay as they
	// were.
	weighed := (k == 0 || !sd.chance(noWeighIn) || d == sd.today) && !at.After(sd.o.Now)
	synced := (!sd.chance(noDevice) || d == sd.today) && !at.After(sd.o.Now)
	steps := math.Round(math.Max(1500, sd.normal(meanSteps, 2600)))
	vs := sd.metrics(d, b, k, shown, weighed, synced, steps)
	for _, v := range vs {
		switch v.Kind {
		case metric.Weight:
			cd.WeightKg = v.Value
		case metric.Steps:
			cd.Steps = v.Value
		case metric.BodyFat:
			cd.BodyFatPct = v.Value
		case metric.LeanMass:
			cd.LeanMassKg = v.Value
		default:
		}
	}
	if len(vs) > 0 {
		if _, err := sd.db.MergeMetrics(ctx, sd.u.ID, importActor, d, vs, sd.written(at)); err != nil {
			return cd, 0, err
		}
	}
	return cd, steps, nil
}

// morningOfToday returns when today's morning is weighed, at: before
// now, when the demo is made before at, as long as that is still today,
// so a demo made early in the day has its morning too.
func (sd *seeder) morningOfToday(at time.Time) time.Time {
	if !at.After(sd.o.Now) {
		return at
	}
	if early := sd.o.Now.Add(-10 * time.Minute); sd.cal.DayOf(early) == sd.today {
		return early
	}
	return at
}

// metrics returns day d's measurements: the weigh-in when weighed, with
// the body fat on Sundays, and the device's steps, sleep, resting heart
// rate and HRV when it synced (today's steps so far).
func (sd *seeder) metrics(d day.Date, b *body, k int, shown float64, weighed, synced bool, steps float64) []metric.Value {
	var vs []metric.Value
	progress := float64(k) / float64(sd.o.Days)
	if weighed {
		kg := math.Round(shown*10) / 10
		vs = append(vs, metric.Value{Kind: metric.Weight, Value: kg, Source: metric.Manual})
		sd.res.WeighIns++
		if d.Weekday() == time.Sunday {
			bf := math.Round((b.fatKg/b.kg*100+sd.normal(0, 0.4))*10) / 10
			vs = append(vs,
				metric.Value{Kind: metric.BodyFat, Value: bf, Source: metric.Import},
				metric.Value{Kind: metric.LeanMass, Value: math.Round(kg*(100-bf)/10) / 10, Source: metric.Import})
		}
	}
	if !synced {
		return vs
	}
	if d == sd.today {
		// Today's steps so far: the share of the waking day, 07:00 to
		// 22:00, that has passed.
		gone := sd.o.Now.Sub(sd.at(d, 7, 0)).Hours() / 15
		steps = math.Round(steps * math.Min(1, math.Max(0, gone)))
	}
	vs = append(vs, metric.Value{Kind: metric.Steps, Value: steps, Source: metric.Import})
	sleep := math.Min(9.5, math.Max(4.5, sd.normal(7.2, 0.6)))
	return append(vs,
		metric.Value{Kind: metric.Sleep, Value: math.Round(sleep*60) / 60, Source: metric.Import},
		metric.Value{Kind: metric.RestingHR, Value: math.Round(sd.normal(60-3*progress, 1.8)), Source: metric.Import},
		metric.Value{Kind: metric.HRV, Value: math.Round(math.Max(15, sd.normal(48+7*progress, 7))), Source: metric.Import})
}

// appetite returns what the user means to eat on day d of target kcal:
// near it, more at the weekend now and then, and a little less once in
// a while.
func (sd *seeder) appetite(d day.Date, kcal float64) float64 {
	want := sd.normal(kcal, 130)
	if wd := d.Weekday(); (wd == time.Friday || wd == time.Saturday) && sd.chance(0.5) {
		want += sd.normal(350, 100)
	}
	if sd.chance(0.05) {
		want -= 250
	}
	return want
}

// meals logs day d's meals, which come to about want kcal, but those
// still to come today; on the day out of calibration the dinner is a
// restaurant's plate. It returns the kcal logged and how many meals.
func (sd *seeder) meals(ctx context.Context, d day.Date, want float64, excluded bool) (float64, int, error) {
	weekend := d.Weekday() == time.Saturday || d.Weekday() == time.Sunday
	var shares float64
	var chosen []slot
	for _, s := range slots {
		// Lunch and dinner are always eaten; the rest now and then.
		if s.chance >= 1 || sd.chance(s.chance) {
			chosen = append(chosen, s)
			shares += s.share
		}
	}
	var kcal float64
	var count int
	for i, s := range chosen {
		minute := s.minute + sd.rng.IntN(s.spread+1)
		if weekend && s.weekendLater {
			minute += 60
		}
		eatenAt := sd.at(d, s.hour, 0).Add(time.Duration(minute) * time.Minute)
		var items []meal.Item
		var err error
		if excluded && s.hour == 19 {
			items, err = sd.restaurantItems()
		} else {
			items, err = sd.items(s.templates[sd.rng.IntN(len(s.templates))], want*s.share/shares)
		}
		if err != nil {
			return 0, 0, err
		}
		if eatenAt.After(sd.o.Now) {
			continue
		}
		m := meal.Meal{ID: meal.NewIDAt(eatenAt), EatenAt: eatenAt, Day: d, Client: meal.ClientWeb, Items: items}
		if i%2 == 1 {
			m.Client = meal.ClientMCP
		}
		if _, _, err := sd.db.CreateMeal(ctx, sd.u.ID, importActor, m, sd.written(eatenAt)); err != nil {
			return 0, 0, err
		}
		for j := range items {
			kcal += items[j].Nutrients.Kcal
		}
		count++
		sd.res.Meals++
	}
	return kcal, count, nil
}

// restaurantItems is the restaurant's plate, an estimate.
func (sd *seeder) restaurantItems() ([]meal.Item, error) {
	it, err := meal.Estimate(restaurant.in(sd.u.Locale), restaurantPlate, "")
	if err != nil {
		return nil, err
	}
	return []meal.Item{it}, nil
}

// items returns the items of a meal of template parts that come to
// about kcal: the parts that scale take what the others leave, within
// half and twice their amount, in steps of 5 g.
func (sd *seeder) items(parts []part, kcal float64) ([]meal.Item, error) {
	var fixed, scaled float64
	for _, p := range parts {
		it, err := sd.item(p, 1)
		if err != nil {
			return nil, err
		}
		if p.scale {
			scaled += it.Nutrients.Kcal
		} else {
			fixed += it.Nutrients.Kcal
		}
	}
	factor := 1.0
	if scaled > 0 {
		factor = math.Min(2, math.Max(0.5, (kcal-fixed)/scaled))
	}
	out := make([]meal.Item, 0, len(parts))
	for _, p := range parts {
		it, err := sd.item(p, factor)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

// item returns part p as an item, its amount times factor when it
// scales.
func (sd *seeder) item(p part, factor float64) (meal.Item, error) {
	f := sd.foods[p.food]
	q := meal.Quantity{Amount: p.amount, Unit: f.Basis.Unit()}
	switch {
	case p.servings:
		q = meal.Quantity{Servings: p.amount, Serving: f.Servings[len(f.Servings)-1].Name}
	case p.scale:
		q.Amount = math.Max(5, math.Round(p.amount*factor/5)*5)
	}
	return meal.FromFood(f, q, "")
}

// checkIn says how day d went now and then, and takes the day out of
// calibration when it is the restaurant's.
func (sd *seeder) checkIn(ctx context.Context, d day.Date, dt plan.DayType, excluded bool) error {
	now := sd.written(sd.at(d, 21, 30))
	if sd.chance(checkedIn) {
		moods := []int{2, 3, 3, 4, 4, 4, 5, 5}
		hungers := []int{2, 3, 3, 4}
		if dt == plan.Training {
			hungers = []int{2, 2, 3, 3}
		}
		c := store.DayCheckIn{Mood: moods[sd.rng.IntN(len(moods))], Hunger: hungers[sd.rng.IntN(len(hungers))]}
		if sd.chance(noteChance) {
			c.Note = checkInNotes[sd.rng.IntN(len(checkInNotes))].in(sd.u.Locale)
		}
		change := func(before store.DayCheckIn) store.DayCheckIn {
			before.Mood, before.Hunger, before.Note = c.Mood, c.Hunger, c.Note
			return before
		}
		if _, err := sd.db.CheckIn(ctx, sd.u.ID, importActor, d, change, now); err != nil {
			return err
		}
		sd.res.CheckIns++
	}
	if excluded {
		return sd.db.ExcludeDay(ctx, sd.u.ID, importActor, d, day.Exclusion{Reason: restaurantReason.in(sd.u.Locale)}, now)
	}
	return nil
}
