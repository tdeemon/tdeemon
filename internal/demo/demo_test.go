package demo

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/tdeemon/tdeemon/internal/domain/calibration"
	"github.com/tdeemon/tdeemon/internal/domain/day"
	"github.com/tdeemon/tdeemon/internal/domain/event"
	"github.com/tdeemon/tdeemon/internal/domain/food"
	"github.com/tdeemon/tdeemon/internal/domain/limits"
	"github.com/tdeemon/tdeemon/internal/domain/meal"
	"github.com/tdeemon/tdeemon/internal/domain/metric"
	"github.com/tdeemon/tdeemon/internal/domain/plan"
	"github.com/tdeemon/tdeemon/internal/domain/proposal"
	"github.com/tdeemon/tdeemon/internal/domain/user"
	"github.com/tdeemon/tdeemon/internal/store"
	"github.com/tdeemon/tdeemon/internal/store/storetest"
)

var zagreb = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Zagreb")
	if err != nil {
		panic(err)
	}
	return loc
}()

// now is the demo data's end in the tests: a Wednesday morning.
var now = time.Date(2026, 11, 4, 10, 30, 0, 0, zagreb)

// seeded stores a user and seeds them with days of data of seed in
// locale, and returns the user and what Seed said.
func seeded(t *testing.T, db *store.DB, days int, seed uint64, locale string) (user.User, Result) {
	t.Helper()

	u := user.User{
		ID: uuid.NewV7(), Email: fmt.Sprintf("demo-%d-%s@example.com", seed, locale), Name: "Demo", Role: user.RoleUser,
		Locale: locale, TimeZone: "Europe/Zagreb", DayStartHour: user.DefaultDayStartHour, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.CreateUser(t.Context(), u, ""); err != nil {
		t.Fatal(err)
	}
	res, err := Seed(t.Context(), db, u, Options{Days: days, Seed: seed, Now: now, Instance: limits.Instance()})
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	return u, res
}

// calibrated measures the TDEE of u's data in db up to the day before
// res.Today, from the store's days as the API reads them.
func calibrated(t *testing.T, db *store.DB, u user.User, res Result) calibration.Estimate {
	t.Helper()

	ctx := t.Context()
	cal, err := day.NewCalendar(u.TimeZone, u.DayStartHour)
	if err != nil {
		t.Fatal(err)
	}
	intake, err := db.IntakeOfDays(ctx, u.ID, res.Start, res.Today)
	if err != nil {
		t.Fatal(err)
	}
	values, err := db.Metrics(ctx, u.ID, res.Start, res.Today)
	if err != nil {
		t.Fatal(err)
	}
	exclusions, err := db.Exclusions(ctx, u.ID, res.Start, res.Today)
	if err != nil {
		t.Fatal(err)
	}
	phases, err := db.Phases(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	days := map[day.Date]calibration.Day{}
	for d := res.Start; !res.Today.Before(d); d = d.AddDays(1) {
		_, excluded := exclusions[d]
		c := calibration.Day{Status: cal.StatusOf(d, now, excluded), Meals: intake[d].Meals, Kcal: intake[d].Kcal}
		for _, v := range values[d] {
			switch v.Kind {
			case metric.Weight:
				c.WeightKg = v.Value
			case metric.Steps:
				c.Steps = v.Value
			case metric.BodyFat:
				c.BodyFatPct = v.Value
			case metric.LeanMass:
				c.LeanMassKg = v.Value
			default:
			}
		}
		days[d] = c
	}
	return calibration.Calibrate(days, phases, res.Today.AddDays(-1), calibration.DefaultParams())
}

// Seed makes 90 days of plausible data: the plan from the first day, a
// weigh-in on most days, 3 to 5 meals on most, every value valid, and a
// TDEE calibration measures near the hidden one.
func TestSeed(t *testing.T) {
	t.Parallel()

	db := storetest.Open(t)
	u, res := seeded(t, db, DefaultDays, DefaultSeed, "en")
	ctx := t.Context()
	today := day.Date{Year: 2026, Month: time.November, Day: 4}
	if res.Today != today || res.Start != today.AddDays(-DefaultDays) {
		t.Errorf("the data run from %s to %s, want %s to %s", res.Start, res.Today, today.AddDays(-DefaultDays), today)
	}

	checkPlan(t, db, u.ID, res)
	checkFoods(t, db, u.ID, res)
	checkMeals(t, db, u.ID, res)
	checkMetrics(t, db, u.ID, res)
	checkIntake(t, db, u.ID, res)

	if res.CheckIns < DefaultDays/3 {
		t.Errorf("%d check-ins, want a third of the days at least", res.CheckIns)
	}
	if ex, err := db.Exclusions(ctx, u.ID, res.Start, res.Today); err != nil || len(ex) != 1 {
		t.Errorf("excluded days %v (%v), want one", ex, err)
	}

	checkCalibration(t, db, u, res)
}

// checkCalibration checks the TDEE calibration measures on u's data, and
// the proposal it makes.
func checkCalibration(t *testing.T, db *store.DB, u user.User, res Result) {
	t.Helper()

	e := calibrated(t, db, u, res)
	if !e.Measured || !e.Trusted || math.Abs(e.TDEE-res.TDEE) > 250 {
		t.Errorf("calibration measures %.0f (measured %v, trusted %v, doubts %v), want within 250 of the hidden %.0f",
			e.TDEE, e.Measured, e.Trusted, e.Doubts, res.TDEE)
	}
	if math.Abs(e.TDEE-res.Measured) > 0.5 {
		t.Errorf("calibration of the store's data measures %.1f, but Seed said %.1f", e.TDEE, res.Measured)
	}
	t.Logf("hidden TDEE %.0f, measured %.0f ± %.0f", res.TDEE, e.TDEE, e.HalfWidth)

	pending, err := db.Proposals(t.Context(), u.ID, 10, now, proposal.Pending)
	if err != nil || len(pending) != 1 || !res.Proposal {
		t.Fatalf("pending proposals %+v (%v), want one", pending, err)
	}
	if p := pending[0]; p.By.Model == "" || len(p.Changes.Targets) != 1 || p.Changes.Targets[0].From != res.Today.AddDays(1) {
		t.Errorf("the proposal %+v, want a coach's target from tomorrow", p)
	}
}

// checkPlan checks the profile and the plan from the first day.
func checkPlan(t *testing.T, db *store.DB, userID uuid.UUID, res Result) {
	t.Helper()

	ctx := t.Context()
	pr, err := db.Profile(ctx, userID)
	if err != nil || !pr.Complete() || pr.Validate(res.Today) != nil {
		t.Errorf("profile %+v (%v), want a complete one", pr, err)
	}
	phases, err := db.Phases(ctx, userID)
	if err != nil || len(phases) != 1 || phases[0].Type != plan.Cut || phases[0].Start != res.Start {
		t.Errorf("phases %+v (%v), want a cut from %s", phases, err, res.Start)
	}
	targets, err := db.Targets(ctx, userID)
	if err != nil || len(targets) != 1 || targets[0].EffectiveFrom != res.Start || targets[0].Training.Kcal <= targets[0].Rest.Kcal {
		t.Errorf("targets %+v (%v), want one from %s with more on training days", targets, err, res.Start)
	}
	if w, err := db.TrainingWeekdays(ctx, userID); err != nil || w.Count() != 4 {
		t.Errorf("training weekdays %v (%v), want 4", w, err)
	}
}

// checkIntake checks that the days logged eat near the targets, the kcal
// within 150 of the weekly average and the protein within a tenth.
func checkIntake(t *testing.T, db *store.DB, userID uuid.UUID, res Result) {
	t.Helper()

	ctx := t.Context()
	intake, err := db.IntakeOfDays(ctx, userID, res.Start, res.Today.AddDays(-1))
	if err != nil {
		t.Fatal(err)
	}
	targets, err := db.Targets(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	want := targets[0].WeeklyAverage(trainingDays().Count())
	var kcal, protein float64
	for _, in := range intake {
		kcal += in.Kcal
		protein += in.Protein
	}
	kcal /= float64(len(intake))
	protein /= float64(len(intake))
	if math.Abs(kcal-want.Kcal) > 150 || math.Abs(protein-want.Protein) > want.Protein/10 {
		t.Errorf("the days logged eat %.0f kcal and %.0f g protein, want near %.0f and %.0f", kcal, protein, want.Kcal, want.Protein)
	}
	t.Logf("the days logged eat %.0f kcal and %.0f g protein, the targets %.0f and %.0f", kcal, protein, want.Kcal, want.Protein)
}

func checkFoods(t *testing.T, db *store.DB, userID uuid.UUID, res Result) {
	t.Helper()

	all, total, err := db.ListFoods(t.Context(), userID, false, "", store.ByName, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != len(foods)+len(recipes) || res.Foods != len(foods) || res.Recipes != len(recipes) {
		t.Errorf("%d foods stored, Seed said %d and %d recipes, want %d and %d", total, res.Foods, res.Recipes, len(foods), len(recipes))
	}
	if len(foods) < 20 || len(foods) > 30 {
		t.Errorf("%d foods, want 20 to 30", len(foods))
	}
	for i := range all {
		f := &all[i]
		if err := f.Validate(); err != nil {
			t.Errorf("food %s: %v", f.Name, err)
		}
		if f.Source != food.Reference && f.Source != food.Recipe {
			t.Errorf("food %s of source %s, want reference or a recipe", f.Name, f.Source)
		}
		if ws := meal.CheckNutrients(f.Name, f.Nutrients); len(ws) > 0 {
			t.Errorf("food %s: %v", f.Name, ws)
		}
	}
}

func checkMeals(t *testing.T, db *store.DB, userID uuid.UUID, res Result) {
	t.Helper()

	cal, err := day.NewCalendar("Europe/Zagreb", user.DefaultDayStartHour)
	if err != nil {
		t.Fatal(err)
	}
	meals, err := db.MealsBetween(t.Context(), userID, cal.Start(res.Start), cal.End(res.Today))
	if err != nil {
		t.Fatal(err)
	}
	if len(meals) != res.Meals {
		t.Errorf("%d meals stored, Seed said %d", len(meals), res.Meals)
	}
	byDay := map[day.Date]int{}
	recipeItems := 0
	for i := range meals {
		m := &meals[i]
		if err := m.Validate(); err != nil {
			t.Errorf("meal at %s: %v", m.EatenAt, err)
		}
		if m.EatenAt.After(now) || cal.DayOf(m.EatenAt) != m.Day {
			t.Errorf("meal at %s of day %s, want one eaten by now, of its day", m.EatenAt, m.Day)
		}
		recipeItems += recipesIn(m.Items)
		byDay[m.Day]++
	}
	checkMealDays(t, byDay, res)
	if recipeItems == 0 {
		t.Error("no meal has a recipe")
	}
}

// recipesIn counts the items of a recipe.
func recipesIn(items []meal.Item) int {
	n := 0
	for i := range items {
		if items[i].Source == food.Recipe {
			n++
		}
	}
	return n
}

// checkMealDays checks how many meals each day has (byDay).
func checkMealDays(t *testing.T, byDay map[day.Date]int, res Result) {
	t.Helper()

	full := 0
	for d := res.Start; d.Before(res.Today); d = d.AddDays(1) {
		if n := byDay[d]; n >= 3 && n <= 5 {
			full++
		} else if n != 0 {
			t.Errorf("%d meals on %s, want 3 to 5, or none on a day not logged", n, d)
		}
	}
	if full < DefaultDays*9/10 {
		t.Errorf("%d days of %d with meals, want 90 %%", full, DefaultDays)
	}
	if byDay[res.Today] == 0 || byDay[res.Today] > 2 {
		t.Errorf("%d meals today at 10:30, want breakfast", byDay[res.Today])
	}
}

func checkMetrics(t *testing.T, db *store.DB, userID uuid.UUID, res Result) {
	t.Helper()

	values, err := db.Metrics(t.Context(), userID, res.Start, res.Today)
	if err != nil {
		t.Fatal(err)
	}
	count := map[metric.Kind]int{}
	for d, vs := range values {
		if err := metric.ValidateDay(vs); err != nil {
			t.Errorf("%s: %v", d, err)
		}
		for _, v := range vs {
			count[v.Kind]++
		}
	}
	if count[metric.Weight] != res.WeighIns || res.WeighIns < DefaultDays*3/4 {
		t.Errorf("%d weigh-ins, Seed said %d, want three days in four", count[metric.Weight], res.WeighIns)
	}
	for _, k := range []metric.Kind{metric.Steps, metric.Sleep, metric.RestingHR, metric.HRV} {
		if count[k] < DefaultDays*9/10 {
			t.Errorf("%d days of %s, want 90 %%", count[k], k)
		}
	}
	if n := count[metric.BodyFat]; n < DefaultDays/7-3 || n > DefaultDays/7+1 {
		t.Errorf("%d days of body fat, want about one a week", n)
	}
}

// fingerprint is what a user's data are, without the ids the store
// makes.
type fingerprint struct {
	Result  Result
	Meals   []string
	Metrics map[day.Date][]metric.Value
}

func fingerprintOf(t *testing.T, db *store.DB, u user.User, res Result) fingerprint {
	t.Helper()

	meals, err := db.MealsBetween(t.Context(), u.ID, now.AddDate(-2, 0, 0), now)
	if err != nil {
		t.Fatal(err)
	}
	fp := fingerprint{Result: res}
	for i := range meals {
		m := &meals[i]
		s := m.EatenAt.UTC().Format(time.RFC3339) + string(m.Client)
		for j := range m.Items {
			it := &m.Items[j]
			s += fmt.Sprintf("|%s %v %s %.3f", it.Name, it.Amount, it.Unit, it.Nutrients.Kcal)
		}
		fp.Meals = append(fp.Meals, s)
	}
	if fp.Metrics, err = db.Metrics(t.Context(), u.ID, res.Start, res.Today); err != nil {
		t.Fatal(err)
	}
	return fp
}

// The same seed gives the same data; another, other data.
func TestSeedIsDeterministic(t *testing.T) {
	t.Parallel()

	data := func(seed uint64) fingerprint {
		db := storetest.Open(t)
		u, res := seeded(t, db, 40, seed, "en")
		return fingerprintOf(t, db, u, res)
	}
	a, b, c := data(7), data(7), data(8)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("seed 7 gives other data the second time:\n%+v\n%+v", a.Result, b.Result)
	}
	if len(a.Meals) == 0 || len(a.Metrics) == 0 {
		t.Errorf("seed 7 gives %d meals and %d days of metrics", len(a.Meals), len(a.Metrics))
	}
	if reflect.DeepEqual(a, c) {
		t.Error("seeds 7 and 8 give the same data")
	}
}

// Over several seeds, in each language, calibration measures a TDEE
// within 250 kcal of the hidden one, and the hidden one is 2500 to 2700.
func TestSeedsCalibrate(t *testing.T) {
	t.Parallel()

	for seed := uint64(1); seed <= 10; seed++ {
		locale := []string{"en", "hr"}[seed%2]
		t.Run(fmt.Sprint(seed, locale), func(t *testing.T) {
			t.Parallel()

			db := storetest.Open(t)
			u, res := seeded(t, db, DefaultDays, seed, locale)
			e := calibrated(t, db, u, res)
			if res.TDEE < 2500 || res.TDEE > 2700 {
				t.Errorf("hidden TDEE %.0f, want 2500 to 2700", res.TDEE)
			}
			if !e.Measured || math.Abs(e.TDEE-res.TDEE) > 250 {
				t.Errorf("calibration measures %.0f (measured %v), want within 250 of the hidden %.0f", e.TDEE, e.Measured, res.TDEE)
			}
			t.Logf("hidden TDEE %.0f, measured %.0f ± %.0f, trusted %v, proposal %v", res.TDEE, e.TDEE, e.HalfWidth, e.Trusted, res.Proposal)
		})
	}
}

// Seed refuses a number of days out of its bounds, and makes the least.
// A fresh demo user has nothing to undo: the user's first Undo takes none
// of the demo data back, the notes for a coach included.
func TestSeedLeavesNothingToUndo(t *testing.T) {
	t.Parallel()

	db := storetest.Open(t)
	u, _ := seeded(t, db, MinDays, DefaultSeed, "en")
	webUser := event.Actor{Kind: event.User, Client: "web"}
	if ev, err := db.UndoLast(t.Context(), u.ID, webUser, now); !errors.Is(err, store.ErrNothingToUndo) {
		t.Errorf("undo on a fresh demo user: %+v, %v; want ErrNothingToUndo", ev, err)
	}
	if notes, err := db.ProfileNotes(t.Context(), u.ID); err != nil || notes == "" {
		t.Errorf("notes after the undo %q (%v), want the demo's", notes, err)
	}
}

func TestSeedDays(t *testing.T) {
	t.Parallel()

	db := storetest.Open(t)
	u := user.User{ID: uuid.NewV7()}
	for _, days := range []int{0, MinDays - 1, MaxDays + 1} {
		if _, err := Seed(t.Context(), db, u, Options{Days: days, Now: now}); !errors.Is(err, ErrDays) {
			t.Errorf("%d days: %v, want ErrDays", days, err)
		}
	}
	if _, err := Seed(t.Context(), db, u, Options{Days: MinDays}); err == nil {
		t.Error("no now: no error")
	}
	_, res := seeded(t, db, MinDays, 3, "hr")
	if res.Start != res.Today.AddDays(-MinDays) || res.Meals < MinDays*3 {
		t.Errorf("%+v, want %d days of meals", res, MinDays)
	}
}

// Today has its morning whatever the seed, and when the demo is made
// before the usual hour of the weigh-in too: the weight and the
// device's sleep, resting heart rate and HRV.
func TestSeedTodayHasItsMorning(t *testing.T) {
	t.Parallel()

	early := time.Date(2026, 11, 4, 6, 15, 0, 0, zagreb)
	for seed := range uint64(12) {
		at := now
		if seed%2 == 1 {
			at = early
		}
		db := storetest.Open(t)
		u := user.User{
			ID: uuid.NewV7(), Email: fmt.Sprintf("today-%d@example.com", seed), Name: "Demo", Role: user.RoleUser,
			Locale: "en", TimeZone: "Europe/Zagreb", DayStartHour: user.DefaultDayStartHour, CreatedAt: at, UpdatedAt: at,
		}
		if err := db.CreateUser(t.Context(), u, ""); err != nil {
			t.Fatal(err)
		}
		res, err := Seed(t.Context(), db, u, Options{Days: MinDays, Seed: seed, Now: at, Instance: limits.Instance()})
		if err != nil {
			t.Fatal(err)
		}
		values, err := db.Metrics(t.Context(), u.ID, res.Today, res.Today)
		if err != nil {
			t.Fatal(err)
		}
		kinds := map[metric.Kind]bool{}
		for _, v := range values[res.Today] {
			kinds[v.Kind] = true
		}
		for _, k := range []metric.Kind{metric.Weight, metric.Sleep, metric.RestingHR, metric.HRV} {
			if !kinds[k] {
				t.Errorf("seed %d at %s: today has no %s", seed, at.Format("15:04"), k)
			}
		}
	}
}
