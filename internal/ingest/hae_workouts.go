package ingest

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/tdeemon/tdeemon/internal/domain/day"
)

// haeStrength are the names of the workouts that make a training day:
// Apple Health's strength training, which Strong writes. Other kinds (a
// walk, a run) are no training day of a plan's split targets.
var haeStrength = map[string]bool{
	"Traditional Strength Training": true,
	"Functional Strength Training":  true,
}

// HAEStrength lists the names of the workouts that make a training day,
// sorted.
func HAEStrength() []string {
	out := make([]string, 0, len(haeStrength))
	for name := range haeStrength {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// HAEWorkouts is what a Health Auto Export payload says of the workouts.
type HAEWorkouts struct {
	// Sent reports that the payload has a list of workouts, even an empty
	// one; without it the payload says nothing of them.
	Sent bool
	// Strength are the days of strength workouts.
	Strength Workouts
	// From and To are the span of days the payload's workouts cover, of
	// any kind, both included; zero when it has none. A sync replaces the
	// days of workouts within it.
	From, To day.Date
	// Skipped are the names of the other kinds of workouts it holds,
	// sorted.
	Skipped []string
}

type haeWorkoutsPayload struct {
	Data *struct {
		Workouts *[]haeWorkout `json:"workouts"`
	} `json:"data"`
}

// haeWorkout is the part of a workout tdeemon reads: its kind and when it
// started (HAE's workouts, v1 and v2).
type haeWorkout struct {
	Name  string `json:"name"`
	Start string `json:"start"`
}

// ParseHAEWorkouts reads the workouts of a Health Auto Export payload: the
// day each starts in, by cal (the user's day start hour, as for Strong's
// export), the days of strength workouts among them, and the span of days
// they cover. A payload without a list of workouts is no error: it says
// nothing of them (Sent is false). A workout whose start does not read
// fails the payload, so a sync never replaces days it could not read.
func ParseHAEWorkouts(body []byte, cal day.Calendar) (HAEWorkouts, error) {
	var p haeWorkoutsPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return HAEWorkouts{}, fmt.Errorf("health auto export payload: %w", err)
	}
	if p.Data == nil || p.Data.Workouts == nil {
		return HAEWorkouts{}, nil
	}
	out := HAEWorkouts{Sent: true, Strength: Workouts{Days: []day.Date{}, perDay: map[day.Date]int{}}, Skipped: []string{}}
	skipped := map[string]bool{}
	for i, w := range *p.Data.Workouts {
		at, err := haeTime(w.Start)
		if err != nil {
			return HAEWorkouts{}, fmt.Errorf("health auto export workout %d (%q): start %q: %w", i+1, w.Name, w.Start, err)
		}
		d := cal.DayOf(at)
		if out.From.IsZero() || d.Before(out.From) {
			out.From = d
		}
		if out.To.IsZero() || out.To.Before(d) {
			out.To = d
		}
		if !haeStrength[w.Name] {
			skipped[w.Name] = true
			continue
		}
		if out.Strength.perDay[d] == 0 {
			out.Strength.Days = append(out.Strength.Days, d)
		}
		out.Strength.perDay[d]++
		out.Strength.Count++
	}
	slices.SortFunc(out.Strength.Days, func(a, b day.Date) int { return a.Compare(b) })
	for name := range skipped {
		out.Skipped = append(out.Skipped, name)
	}
	slices.Sort(out.Skipped)
	return out, nil
}

// haeTime reads a date in one of HAE's forms.
func haeTime(s string) (time.Time, error) {
	for _, layout := range haeTimes {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("want a date such as 2006-01-02 15:04:05 -0700")
}
