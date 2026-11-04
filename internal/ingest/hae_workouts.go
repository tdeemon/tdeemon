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

// Workouts are the days of workouts a payload holds, in order, and how
// many workouts they are; after UpTo, the days after today it left out,
// in order too.
type Workouts struct {
	Count  int
	Days   []day.Date
	Future []day.Date
	// perDay is how many workouts each day holds.
	perDay map[day.Date]int
}

// UpTo returns w with the days after today left out, as a measurement's
// are (Result.UpTo): a clock set wrong, or a time zone, would put a
// workout on a day to come, and make it a training day it is not. The
// days left out are its Future, and their workouts are no longer counted.
func (w Workouts) UpTo(today day.Date) Workouts {
	out := Workouts{Days: []day.Date{}, perDay: map[day.Date]int{}}
	for _, d := range w.Days {
		if today.Before(d) {
			out.Future = append(out.Future, d)
			continue
		}
		out.Days = append(out.Days, d)
		out.Count += w.perDay[d]
		out.perDay[d] = w.perDay[d]
	}
	return out
}

// HAEWorkouts is what a Health Auto Export payload says of the workouts.
type HAEWorkouts struct {
	// Sent reports that the payload has a list of workouts, even an empty
	// one; without it the payload says nothing of them.
	Sent bool
	// Strength are the days of strength workouts.
	Strength Workouts
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
// export), and the days of strength workouts among them. A payload
// without a list of workouts is no error: it says nothing of them (Sent
// is false). A workout whose start does not read fails the payload, so
// none of it is stored.
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
