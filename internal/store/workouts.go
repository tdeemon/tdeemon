package store

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"time"
	"uuid"

	"github.com/tdeemon/tdeemon/internal/domain/day"
	"github.com/tdeemon/tdeemon/internal/domain/event"
)

// WorkoutSource is where the days of workouts come from.
type WorkoutSource string

// Workout sources.
const (
	// Strong is the Strong app's export, its whole history each time.
	Strong WorkoutSource = "strong"
	// HAE is Health Auto Export's workouts, Apple Health's: a span of
	// days each time.
	HAE WorkoutSource = "hae"
)

// workoutDaysState is the state of a source's days of workouts.
type workoutDaysState struct {
	Days []string `json:"days"`
}

// SetWorkoutDays replaces the days source recorded a workout on with ds,
// as an import of source's whole history, and returns the days it added
// and removed, in order. A day of a workout is a training day unless the
// check-in's toggle says otherwise (plan.TypeOf), so the toggle is not
// touched. The same days again record nothing (record), and a day the
// table cannot hold, such as the zero Date, fails it.
func (db *DB) SetWorkoutDays(ctx context.Context, userID uuid.UUID, a event.Actor, source WorkoutSource, ds []day.Date,
	now time.Time,
) (added, removed []day.Date, err error) {
	if source != Strong {
		return nil, nil, fmt.Errorf("invalid workout source %q: want %q", source, Strong)
	}
	return db.setWorkoutDays(ctx, userID, a, source, ds, func(day.Date) bool { return true }, now)
}

// SetWorkoutDaysBetween replaces the days source recorded a workout on
// from from to to, both included, with ds, as an import of that span:
// source's days outside it stay. It returns the days it added and
// removed, in order, as SetWorkoutDays does; a day of ds outside the span
// fails it.
func (db *DB) SetWorkoutDaysBetween(ctx context.Context, userID uuid.UUID, a event.Actor, source WorkoutSource,
	from, to day.Date, ds []day.Date, now time.Time,
) (added, removed []day.Date, err error) {
	if source != HAE {
		return nil, nil, fmt.Errorf("invalid workout source %q: want %q", source, HAE)
	}
	if to.Before(from) {
		return nil, nil, fmt.Errorf("set workout days: the span %s to %s ends before it starts", from, to)
	}
	in := func(d day.Date) bool { return !d.Before(from) && !to.Before(d) }
	for _, d := range ds {
		if !in(d) {
			return nil, nil, fmt.Errorf("set workout days: %s is outside %s to %s", d, from, to)
		}
	}
	return db.setWorkoutDays(ctx, userID, a, source, ds, in, now)
}

// setWorkoutDays makes source's days that in holds the days of ds, and
// records the change of all source's days.
func (db *DB) setWorkoutDays(ctx context.Context, userID uuid.UUID, a event.Actor, source WorkoutSource, ds []day.Date,
	in func(day.Date) bool, now time.Time,
) (added, removed []day.Date, err error) {
	if err := a.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid actor: %w", err)
	}
	want := map[day.Date]bool{}
	for _, d := range ds {
		want[d] = true
	}
	err = db.WriteTx(ctx, func(tx *sql.Tx) error {
		have, err := workoutDaysOf(ctx, tx, userID, source)
		if err != nil {
			return err
		}
		// The days outside the span stay as they are.
		for _, d := range have {
			if !in(d) {
				want[d] = true
			}
		}
		added, removed = diffDays(have, want)
		if err := putWorkoutDays(ctx, tx, userID, source, added, removed); err != nil {
			return err
		}
		after, err := workoutDaysOf(ctx, tx, userID, source)
		if err != nil {
			return err
		}
		_, err = record(ctx, tx, userID, a, event.WorkoutsSet, string(source), stateOfDays(have), stateOfDays(after), now)
		return err
	})
	if err != nil {
		return nil, nil, fmt.Errorf("set workout days: %w", err)
	}
	return added, removed, nil
}

// diffDays returns the days of want that have lacks, and the days of have
// that want lacks, each in order.
func diffDays(have []day.Date, want map[day.Date]bool) (added, removed []day.Date) {
	had := map[day.Date]bool{}
	for _, d := range have {
		had[d] = true
		if !want[d] {
			removed = append(removed, d)
		}
	}
	for d := range want {
		if !had[d] {
			added = append(added, d)
		}
	}
	slices.SortFunc(added, func(a, b day.Date) int { return strings.Compare(a.String(), b.String()) })
	return added, removed
}

func putWorkoutDays(ctx context.Context, tx *sql.Tx, userID uuid.UUID, source WorkoutSource, added, removed []day.Date) error {
	for _, d := range removed {
		if _, err := tx.ExecContext(ctx, `DELETE FROM workout_days WHERE user_id = ? AND source = ? AND day = ?`,
			userID.String(), string(source), d.String()); err != nil {
			return err
		}
	}
	for _, d := range added {
		if _, err := tx.ExecContext(ctx, `INSERT INTO workout_days (user_id, source, day) VALUES (?, ?, ?)`,
			userID.String(), string(source), d.String()); err != nil {
			return err
		}
	}
	return nil
}

func stateOfDays(ds []day.Date) workoutDaysState {
	s := workoutDaysState{Days: make([]string, len(ds))}
	for i, d := range ds {
		s.Days[i] = d.String()
	}
	return s
}

// workoutDaysOf returns the days source recorded a workout on, in order.
func workoutDaysOf(ctx context.Context, tx *sql.Tx, userID uuid.UUID, source WorkoutSource) ([]day.Date, error) {
	rows, err := tx.QueryContext(ctx, `SELECT day FROM workout_days WHERE user_id = ? AND source = ? ORDER BY day`,
		userID.String(), string(source))
	if err != nil {
		return nil, err
	}
	return scanDays(rows)
}

// WorkoutDays returns the days from from to to on which any source
// recorded a workout.
func (db *DB) WorkoutDays(ctx context.Context, userID uuid.UUID, from, to day.Date) (map[day.Date]bool, error) {
	rows, err := db.read.QueryContext(ctx, `SELECT DISTINCT day FROM workout_days
		WHERE user_id = ? AND day BETWEEN ? AND ? ORDER BY day`, userID.String(), from.String(), to.String())
	if err != nil {
		return nil, fmt.Errorf("workout days: %w", err)
	}
	ds, err := scanDays(rows)
	if err != nil {
		return nil, fmt.Errorf("workout days: %w", err)
	}
	out := make(map[day.Date]bool, len(ds))
	for _, d := range ds {
		out[d] = true
	}
	return out, nil
}

func scanDays(rows *sql.Rows) ([]day.Date, error) {
	defer func() { _ = rows.Close() }()
	var out []day.Date
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		d, err := day.ParseDate(s)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
