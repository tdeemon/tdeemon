package ingest_test

import (
	"slices"
	"testing"

	"github.com/tdeemon/tdeemon/internal/domain/day"
	"github.com/tdeemon/tdeemon/internal/domain/metric"
	"github.com/tdeemon/tdeemon/internal/ingest"
)

// UpTo keeps the days up to today, today among them, and rejects each
// value of a day after it, after what was rejected already.
func TestUpTo(t *testing.T) {
	t.Parallel()

	today := day.Date{Year: 2026, Month: 10, Day: 22}
	steps := func(v float64) []metric.Value {
		return []metric.Value{{Kind: metric.Steps, Value: v, Source: metric.Import}}
	}
	before := ingest.Rejected{RawDate: "yesterday", Kind: metric.Steps, Value: 1, Reason: "the date"}
	r := ingest.Result{
		Days:     []ingest.Day{{Date: today.AddDays(-1), Values: steps(10)}, {Date: today, Values: steps(20)}, {Date: today.AddDays(1), Values: steps(30)}},
		Rejected: []ingest.Rejected{before},
		Ignored:  []string{"mood"},
	}
	got := r.UpTo(today)
	if len(got.Days) != 2 || got.Days[0].Date != today.AddDays(-1) || got.Days[1].Date != today {
		t.Errorf("days = %+v, want yesterday and today", got.Days)
	}
	want := []ingest.Rejected{before, {Date: today.AddDays(1), Kind: metric.Steps, Value: 30, Reason: ingest.FutureReason}}
	if !slices.Equal(got.Rejected, want) || !slices.Equal(got.Ignored, []string{"mood"}) {
		t.Errorf("rejected %+v, ignored %v; want %+v and mood", got.Rejected, got.Ignored, want)
	}
}
