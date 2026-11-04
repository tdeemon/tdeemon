// Package demo fills a new user's account with months of synthetic data,
// for screenshots and for trying tdeemon without one's own: a profile, a
// cut with its targets, foods and recipes, meals, measurements and
// check-ins, and a proposal of a coach's that waits. The data are made
// up and consistent with one another: the weight follows the energy
// balance of what the user logged against a hidden TDEE, so calibration
// measures one near it.
//
// Every write goes through the store's methods, so it keeps the rules a
// client's would. The same seed and the same now give the same data.
package demo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/tdeemon/tdeemon/internal/domain/calibration"
	"github.com/tdeemon/tdeemon/internal/domain/day"
	"github.com/tdeemon/tdeemon/internal/domain/event"
	"github.com/tdeemon/tdeemon/internal/domain/food"
	"github.com/tdeemon/tdeemon/internal/domain/limits"
	"github.com/tdeemon/tdeemon/internal/domain/user"
	"github.com/tdeemon/tdeemon/internal/store"
)

// The days of data Seed makes: enough for calibration to measure a TDEE
// past a phase's first weeks (calibration.DefaultParams), and at most a
// year.
const (
	MinDays     = 30
	MaxDays     = 365
	DefaultDays = 90
)

// DefaultSeed is the seed of the demo data when none is given, so the
// same command makes the same data.
const DefaultSeed = 1

// Options are what Seed makes.
type Options struct {
	// Days is how many days before today the data start, and the cut
	// with them, MinDays to MaxDays; today has the data of its morning.
	Days int
	// Seed picks the data: the same seed, now and user's calendar give
	// the same data.
	Seed uint64
	// Now is the instant the data end, today being its day in the
	// user's calendar.
	Now time.Time
	// Instance is the instance's limits, which the plan and the proposal
	// keep, as the user's would.
	Instance limits.Limits
}

// Result says what Seed made.
type Result struct {
	// Start is the first day of the data and the cut, and Today the day
	// of Options.Now.
	Start, Today day.Date
	// TDEE is the hidden one the weight follows: the average energy the
	// user spent a day from Start to the day before Today.
	TDEE float64
	// Measured is calibration's TDEE on the data, up to the day before
	// Today, 0 when it measures none.
	Measured                                  float64
	Foods, Recipes, Meals, WeighIns, CheckIns int
	// Proposal is true when a coach's proposal waits: one is made when
	// calibration trusts its TDEE.
	Proposal bool
}

// The actors of the writes. An import's writes are left by undo, so the
// demo user's first undo takes none of the demo data back; a change of
// the plan is the user's, as a proposal's is.
var (
	importActor = event.Actor{Kind: event.Import, Client: "demo"}
	userActor   = event.Actor{Kind: event.User, Client: "demo"}
	coachActor  = event.Actor{Kind: event.User, Client: "mcp", Model: "claude-sonnet-4-5"}
)

// ErrDays reports a number of days outside MinDays to MaxDays.
var ErrDays = fmt.Errorf("want %d to %d days", MinDays, MaxDays)

// seeder holds what Seed works with as it goes.
type seeder struct {
	db    *store.DB
	u     user.User
	o     Options
	rng   *rand.Rand
	cal   day.Calendar
	start day.Date
	today day.Date
	// foods are the user's foods and recipes' foods as stored, by key.
	foods map[string]food.Food
	// days are the data as calibration reads them, built as they are
	// written.
	days map[day.Date]calibration.Day
	// w0 is the weight the cut starts at, and base the TDEE of an
	// average day, which the user does not know.
	w0, base float64
	res      Result
}

// Seed writes Options.Days of data for u, a user already stored with no
// data, through db, and says what it wrote. It stops at the first
// write that fails, leaving what it wrote: the caller deletes the user.
func Seed(ctx context.Context, db *store.DB, u user.User, o Options) (Result, error) {
	if o.Days < MinDays || o.Days > MaxDays {
		return Result{}, fmt.Errorf("%d days: %w", o.Days, ErrDays)
	}
	if o.Now.IsZero() {
		return Result{}, errors.New("no time to end the data at")
	}
	cal, err := day.NewCalendar(u.TimeZone, u.DayStartHour)
	if err != nil {
		return Result{}, err
	}
	sd := &seeder{
		db: db, u: u, o: o, cal: cal,
		// The second word of the source is fixed, so the seed alone
		// picks the data.
		rng:   rand.New(rand.NewPCG(o.Seed, 0x7464656d6f6e)), //nolint:gosec // G404: demo data, not a secret
		foods: map[string]food.Food{},
		days:  map[day.Date]calibration.Day{},
	}
	sd.w0 = math.Round((86+4*sd.rng.Float64())*10) / 10
	sd.base = float64(2540 + sd.rng.IntN(121))
	sd.today = cal.DayOf(o.Now)
	sd.start = sd.today.AddDays(-o.Days)
	sd.res.Start, sd.res.Today = sd.start, sd.today
	for _, step := range []func(context.Context) error{sd.profile, sd.saveFoods, sd.saveRecipes, sd.plan, sd.history, sd.propose} {
		if err := step(ctx); err != nil {
			return sd.res, err
		}
	}
	return sd.res, nil
}

// at returns the instant of the wall clock hour:minute on d, in the
// user's time zone.
func (sd *seeder) at(d day.Date, hour, minute int) time.Time {
	return time.Date(d.Year, d.Month, d.Day, hour, minute, 0, 0, sd.cal.Location)
}

// written returns when a write of something at t is recorded: a few
// minutes after it, and never after now.
func (sd *seeder) written(t time.Time) time.Time {
	w := t.Add(time.Duration(2+sd.rng.IntN(10)) * time.Minute)
	if w.After(sd.o.Now) {
		return sd.o.Now
	}
	return w
}

// normal returns a normally distributed number of mean mu and standard
// deviation sigma.
func (sd *seeder) normal(mu, sigma float64) float64 { return mu + sigma*sd.rng.NormFloat64() }

// chance returns true with probability p.
func (sd *seeder) chance(p float64) bool { return sd.rng.Float64() < p }
