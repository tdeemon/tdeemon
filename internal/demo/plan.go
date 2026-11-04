package demo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/zarnautovic/tdeemon/internal/domain/calibration"
	"github.com/zarnautovic/tdeemon/internal/domain/day"
	"github.com/zarnautovic/tdeemon/internal/domain/limits"
	"github.com/zarnautovic/tdeemon/internal/domain/plan"
	"github.com/zarnautovic/tdeemon/internal/domain/profile"
	"github.com/zarnautovic/tdeemon/internal/domain/proposal"
)

// demoProfile is the demo user: a man of 181 cm, born in 1991.
var demoProfile = profile.Profile{
	Sex:       profile.Male,
	BirthDate: day.Date{Year: 1991, Month: time.March, Day: 12},
	HeightCm:  181,
}

// The cut the demo user is on, and the step down of the targets a
// coach proposes.
const (
	cutRate   = -0.5
	goalLoss  = 8
	stepKcal  = 100
	slowCut   = -cutRate // a loss slower than the plan's gets the proposal
	trainKcal = 2450
	restKcal  = 2100
	trainFat  = 70
	restFat   = 65
)

// trainingDays are the demo user's training weekdays.
func trainingDays() plan.Weekdays {
	w, _ := plan.WeekdaysOf(time.Monday, time.Tuesday, time.Thursday, time.Saturday) // valid weekdays
	return w
}

// demoNotes are the notes the demo user keeps for a coach.
var demoNotes = name{
	"Trains four times a week, an upper and lower split.\nCooks in batches on Sundays and likes simple meals.\nNo allergies.",
	"Trenira četiri puta tjedno, gornji i donji dio tijela.\nNedjeljom kuha za nekoliko dana i voli jednostavne obroke.\nNema alergija.",
}

// profile stores the demo user's profile and their notes for a coach.
func (sd *seeder) profile(ctx context.Context) error {
	now := sd.at(sd.start, 8, 0)
	if err := sd.db.SetProfile(ctx, sd.u.ID, userActor, demoProfile, sd.start, now); err != nil {
		return fmt.Errorf("profile: %w", err)
	}
	if err := sd.db.SetProfileNotes(ctx, sd.u.ID, userActor, demoNotes.in(sd.u.Locale), now); err != nil {
		return fmt.Errorf("notes: %w", err)
	}
	return nil
}

// target returns the demo user's targets from day from: protein of
// 2.2 g/kg of the weight the cut starts at, in steps of 5 g.
func (sd *seeder) target(from day.Date) plan.Target {
	protein := math.Round(2.2*sd.w0/5) * 5
	return plan.Target{
		EffectiveFrom: from,
		Training:      plan.DayTarget{Kcal: trainKcal, Protein: protein, Fat: trainFat},
		Rest:          plan.DayTarget{Kcal: restKcal, Protein: protein, Fat: restFat},
	}
}

// phase returns the demo user's cut.
func (sd *seeder) phase() plan.Phase {
	return plan.Phase{Type: plan.Cut, Start: sd.start, RatePctWeek: cutRate, GoalWeight: new(math.Round(sd.w0 - goalLoss))}
}

// plan makes the user's plan on the first day, as their own change in
// the app: the cut, its targets and the training weekdays, held to the
// limits as any change of the plan.
func (sd *seeder) plan(ctx context.Context) error {
	ph, t, w := sd.phase(), sd.target(sd.start), trainingDays()
	d := proposal.Draft{Rationale: proposal.ChangedInTheApp, Changes: proposal.Changes{
		Phases:           []proposal.PhaseChange{{Start: sd.start, After: &ph}},
		Targets:          []proposal.TargetChange{{From: sd.start, After: &t}},
		TrainingWeekdays: &proposal.WeekdaysChange{After: w},
	}}
	state := limits.State{Today: sd.start, Profile: demoProfile, TrendKg: sd.w0, TrainingDays: w}
	check, err := sd.check(ctx, state, nil, nil, 0)
	if err != nil {
		return err
	}
	if _, err := sd.db.ChangePlan(ctx, sd.u.ID, userActor, d, sd.start, sd.at(sd.start, 8, 10), check); err != nil {
		return fmt.Errorf("plan: %w", err)
	}
	return nil
}

// check returns the check a change of the plan keeps, as the API's: the
// limits of the instance and the user's, at state; it returns tdee, the
// TDEE calibration trusts.
func (sd *seeder) check(ctx context.Context, state limits.State, phases []plan.Phase, targets []plan.Target, tdee float64) (func(proposal.Changes) (float64, error), error) {
	user, err := sd.db.UserLimits(ctx, sd.u.ID)
	if err != nil {
		return nil, err
	}
	levels := limits.Levels{Instance: sd.o.Instance, User: user}
	return func(c proposal.Changes) (float64, error) {
		var errs []error
		for _, ck := range c.Check(state, levels, phases, targets) {
			for _, v := range ck.Violations {
				errs = append(errs, fmt.Errorf("%s from %s: %s, %v past %v", ck.Of, ck.Day, v.Rule, v.Proposed, v.Limit))
			}
		}
		if errs != nil {
			return 0, fmt.Errorf("the limits refuse it: %w", errors.Join(errs...))
		}
		return tdee, nil
	}, nil
}

// propose measures the TDEE on the data, as the app does, and when
// calibration trusts it and the cut runs slower than planned, makes a
// coach's proposal that waits: both targets 100 kcal lower from
// tomorrow.
func (sd *seeder) propose(ctx context.Context) error {
	user, err := sd.db.UserLimits(ctx, sd.u.ID)
	if err != nil {
		return err
	}
	lim := limits.Effective(user, sd.o.Instance)
	params := calibration.DefaultParams()
	params.LoggedDays, params.WeighIns = lim.LoggedDays, lim.WeighIns
	ph, t := sd.phase(), sd.target(sd.start)
	phases, targets := []plan.Phase{ph}, []plan.Target{t}
	yesterday := sd.today.AddDays(-1)
	e := calibration.Calibrate(sd.days, phases, yesterday, params)
	if !e.Measured {
		return nil
	}
	sd.res.Measured = e.TDEE
	loss := -plan.RatePctWeek(e.KgPerWeek, e.TrendKg)
	if !e.Trusted || e.BeforePhase || loss >= slowCut {
		return nil
	}
	next := t.ShiftKcal(-stepKcal)
	next.EffectiveFrom = sd.today.AddDays(1)
	d := proposal.Draft{
		Rationale: sd.rationale(e, loss),
		Window:    &proposal.Window{From: e.From, To: e.To},
		Changes:   proposal.Changes{Targets: []proposal.TargetChange{{From: next.EffectiveFrom, After: &next}}},
	}
	state := calibration.StateOf(e, sd.today, demoProfile, &ph, &t, trainingDays())
	check, err := sd.check(ctx, state, phases, targets, e.TDEE)
	if err != nil {
		return err
	}
	if _, err := sd.db.CreateProposal(ctx, sd.u.ID, coachActor, d, sd.today, sd.o.Now, check); err != nil {
		return fmt.Errorf("proposal: %w", err)
	}
	sd.res.Proposal = true
	return nil
}

// rationale is why the coach proposes the step, in the user's language.
func (sd *seeder) rationale(e calibration.Estimate, loss float64) string {
	num := func(v float64, prec int) string {
		s := strconv.FormatFloat(v, 'f', prec, 64)
		if sd.u.Locale == "hr" {
			s = strings.Replace(s, ".", ",", 1)
		}
		return s
	}
	days, tdee, rate, planned := strconv.Itoa(e.Days), num(math.Round(e.TDEE), 0), num(loss, 2), num(-cutRate, 1)
	if sd.u.Locale == "hr" {
		return "U zadnjih " + days + " dana gubiš " + rate + " % težine tjedno, sporije od planiranih " + planned +
			" %, uz izmjereni TDEE od " + tdee + " kcal.\n" +
			"Niži cilj za 100 kcal na oba tipa dana vraća deficit na tempo plana.\n" +
			"Proteini i masti ostaju isti; razliku nose ugljikohidrati."
	}
	return "Over the last " + days + " days you lost " + rate + " % of your weight a week, slower than the " + planned +
		" % the cut plans, at a measured TDEE of " + tdee + " kcal.\n" +
		"Lowering both day types by 100 kcal brings the deficit back to the plan's pace.\n" +
		"Protein and fat stay as they are; the carbs take the change."
}
