package ingest

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tdeemon/tdeemon/internal/domain/day"
)

// The workouts in testdata/hae/workouts.json are made up, in the shape of
// HAE's Workouts export (v2): its other fields are there to be ignored.

func zagreb4(t *testing.T) day.Calendar {
	t.Helper()
	c, err := day.NewCalendar("Europe/Zagreb", 4)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func dates(t *testing.T, ss ...string) []day.Date {
	t.Helper()
	out := make([]day.Date, len(ss))
	for i, s := range ss {
		d, err := day.ParseDate(s)
		if err != nil {
			t.Fatal(err)
		}
		out[i] = d
	}
	return out
}

// Strength workouts make their days, by the day they start in on the
// user's clock: one at 01:30 belongs to the day before. Other kinds are
// skipped, and named.
func TestParseHAEWorkouts(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join("testdata", "hae", "workouts.json"))
	if err != nil {
		t.Fatal(err)
	}
	w, err := ParseHAEWorkouts(body, zagreb4(t))
	if err != nil {
		t.Fatal(err)
	}
	if !w.Sent {
		t.Error("Sent = false for a payload with workouts")
	}
	if want := dates(t, "2026-10-12", "2026-10-14", "2026-10-16"); !slices.Equal(w.Strength.Days, want) {
		t.Errorf("days = %v, want %v", w.Strength.Days, want)
	}
	if w.Strength.Count != 4 {
		t.Errorf("count = %d, want 4 strength workouts", w.Strength.Count)
	}
	if !slices.Equal(w.Skipped, []string{"Running", "Walking"}) {
		t.Errorf("skipped = %v", w.Skipped)
	}
	// UpTo leaves out the days after today, as for Strong.
	if up := w.Strength.UpTo(dates(t, "2026-10-14")[0]); !slices.Equal(up.Days, dates(t, "2026-10-12", "2026-10-14")) ||
		!slices.Equal(up.Future, dates(t, "2026-10-16")) || up.Count != 2 {
		t.Errorf("UpTo = %+v", up)
	}
}

// A payload without a list of workouts says nothing of them; an empty
// list has no days; a start that does not read fails it all.
func TestParseHAEWorkoutsEdges(t *testing.T) {
	t.Parallel()

	cal := zagreb4(t)
	for name, body := range map[string]string{
		"metrics alone": `{"data": {"metrics": []}}`,
		"no data":       `{}`,
	} {
		w, err := ParseHAEWorkouts([]byte(body), cal)
		if err != nil || w.Sent {
			t.Errorf("%s: %+v, %v; want nothing sent", name, w, err)
		}
	}
	w, err := ParseHAEWorkouts([]byte(`{"data": {"workouts": []}}`), cal)
	if err != nil || !w.Sent || len(w.Strength.Days) != 0 {
		t.Errorf("an empty list: %+v, %v", w, err)
	}
	for name, body := range map[string]string{
		"a start that does not read": `{"data": {"workouts": [{"name": "Traditional Strength Training", "start": "yesterday"}]}}`,
		"not JSON":                   `{"data":`,
	} {
		if _, err := ParseHAEWorkouts([]byte(body), cal); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestHAEStrength(t *testing.T) {
	t.Parallel()

	if got := HAEStrength(); !slices.Equal(got, []string{"Functional Strength Training", "Traditional Strength Training"}) {
		t.Errorf("HAEStrength = %v", got)
	}
}
