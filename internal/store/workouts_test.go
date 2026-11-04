package store_test

import (
	"reflect"
	"testing"
	"uuid"

	"github.com/tdeemon/tdeemon/internal/domain/day"
	"github.com/tdeemon/tdeemon/internal/domain/event"
	"github.com/tdeemon/tdeemon/internal/store"
)

var haeActor = event.Actor{Kind: event.Import, Client: "hae"}

// addWorkouts adds days of workouts from Health Auto Export to the user's
// and returns the days it added.
func addWorkouts(t *testing.T, db *store.DB, userID uuid.UUID, ds ...day.Date) []day.Date {
	t.Helper()
	added, err := db.AddWorkoutDays(t.Context(), userID, haeActor, store.HAE, ds, t0)
	if err != nil {
		t.Fatalf("AddWorkoutDays: %v", err)
	}
	return added
}

// An import adds days and removes none, and leaves another user's; the
// days come back from one day to another, these included.
func TestAddWorkoutDays(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	d := day.DateOf(t0)
	addWorkouts(t, db, ivo, d.AddDays(-3))
	if added := addWorkouts(t, db, ana, d.AddDays(-2), d, d.AddDays(-5), d); !reflect.DeepEqual(added, []day.Date{d.AddDays(-5), d.AddDays(-2), d}) {
		t.Errorf("the first import added %v", added)
	}
	if added := addWorkouts(t, db, ana, d.AddDays(-9)); !reflect.DeepEqual(added, []day.Date{d.AddDays(-9)}) {
		t.Errorf("the second import added %v", added)
	}
	got, err := db.WorkoutDays(t.Context(), ana, d.AddDays(-30), d)
	want := map[day.Date]bool{d.AddDays(-9): true, d.AddDays(-5): true, d.AddDays(-2): true, d: true}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("days = %v, %v; want %v (both imports')", got, err, want)
	}
	if got, err := db.WorkoutDays(t.Context(), ana, d.AddDays(-2), d.AddDays(-1)); err != nil || !reflect.DeepEqual(got, map[day.Date]bool{d.AddDays(-2): true}) {
		t.Errorf("two days = %v, %v", got, err)
	}
	es, err := db.Events(t.Context(), ana, 1)
	if err != nil || len(es) != 1 || es[0].Action != event.WorkoutsSet || es[0].Entity != "hae" || es[0].Actor != haeActor {
		t.Errorf("the event = %+v, %v", es, err)
	}
	n := countRows(t, db, "events")
	if added := addWorkouts(t, db, ana, d.AddDays(-2), d.AddDays(-9)); added != nil || countRows(t, db, "events") != n {
		t.Errorf("the same days again added %v", added)
	}
	if got, _ := db.WorkoutDays(t.Context(), ivo, d.AddDays(-30), d); !reflect.DeepEqual(got, map[day.Date]bool{d.AddDays(-3): true}) {
		t.Errorf("Ivo's days = %v", got)
	}
}

// The days of workouts leave the check-in's toggle alone, and undo passes
// over them, even when no import wrote them.
func TestWorkoutDaysLeaveTheRest(t *testing.T) {
	t.Parallel()

	db, ana, _ := withTwoUsers(t)
	d := day.DateOf(t0)
	if err := db.SetTrainingDay(t.Context(), ana, testActor, d.AddDays(-2), new(false), t0); err != nil {
		t.Fatal(err)
	}
	addWorkouts(t, db, ana, d.AddDays(-2), d)
	if toggles, _ := db.TrainingDayToggles(t.Context(), ana, d.AddDays(-7), d); !reflect.DeepEqual(toggles, map[day.Date]bool{d.AddDays(-2): false}) {
		t.Errorf("toggles = %v", toggles)
	}
	if _, err := db.AddWorkoutDays(t.Context(), ana, testActor, store.HAE, []day.Date{d.AddDays(-1)}, t0); err != nil {
		t.Fatal(err)
	}
	if e := undo(t, db, ana); e.Action != event.TrainingDay {
		t.Errorf("undo took back %s, want the toggle", e.Action)
	}
	if got, _ := db.WorkoutDays(t.Context(), ana, d.AddDays(-30), d); len(got) != 3 {
		t.Errorf("days after undo = %v", got)
	}
}

func TestAddWorkoutDaysRefuses(t *testing.T) {
	t.Parallel()

	db, ana, _ := withTwoUsers(t)
	d := day.DateOf(t0)
	for name, err := range map[string]error{
		"no actor":   first(db.AddWorkoutDays(t.Context(), ana, event.Actor{}, store.HAE, []day.Date{d}, t0)),
		"a source":   first(db.AddWorkoutDays(t.Context(), ana, haeActor, "strong", []day.Date{d}, t0)),
		"a zero day": first(db.AddWorkoutDays(t.Context(), ana, haeActor, store.HAE, []day.Date{{}}, t0)),
	} {
		if err == nil {
			t.Errorf("%s: AddWorkoutDays succeeded", name)
		}
	}
	if countRows(t, db, "workout_days") != 0 || countRows(t, db, "events") != 0 {
		t.Error("a refused import wrote")
	}
	if added, err := db.AddWorkoutDays(t.Context(), ana, haeActor, store.HAE, nil, t0); err != nil || added != nil || countRows(t, db, "events") != 0 {
		t.Errorf("no days: added %v, %v", added, err)
	}
}

func first(_ []day.Date, err error) error { return err }
