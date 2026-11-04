package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tdeemon/tdeemon/internal/auth"
	"github.com/tdeemon/tdeemon/internal/domain/day"
	"github.com/tdeemon/tdeemon/internal/domain/event"
	"github.com/tdeemon/tdeemon/internal/ingest"
	"github.com/tdeemon/tdeemon/internal/store"
)

// maxIngestBytes bounds the body of an ingest: a sync, or a larger export
// split in the app.
const maxIngestBytes = 10 << 20

// ingestReadTimeout is how long HAE's body may take to arrive, on a
// phone's slow upload: longer than the server's own deadline. The simple
// form, a day's values, keeps huma's 5 s.
const ingestReadTimeout = 2 * time.Minute

type ingestHAEInput struct {
	RawBody []byte `contentType:"application/json"`
}

type ingestMetricsInput struct {
	Body struct {
		Date    string             `json:"date" format:"date" example:"2026-10-21" doc:"The day the values belong to."`
		Metrics map[string]float64 `json:"metrics" doc:"Values by metric name (docs: DESIGN.md §4): weight (kg), body_fat (%), lean_mass (kg), steps, active_kcal, basal_kcal, sleep_h (hours), rhr (bpm), hrv (ms), vo2max, exercise_min."`
	}
}

type ingestChange struct {
	Metric string   `json:"metric"`
	Value  float64  `json:"value"`
	Before *float64 `json:"before,omitempty" doc:"The day's value before, of an import; absent when the day had none."`
}

type ingestDay struct {
	Date    string         `json:"date" format:"date"`
	Changed []ingestChange `json:"changed" nullable:"false" doc:"The values the day keeps now; a value the day held already, or one a manual value or the day's first weighing keeps out, is not here."`
}

type ingestRejected struct {
	Date    string  `json:"date,omitempty" format:"date" doc:"Absent when the date itself did not read, or the whole metric was left out."`
	RawDate string  `json:"raw_date,omitempty" doc:"The date as the source wrote it, when it did not read."`
	Metric  string  `json:"metric"`
	Value   float64 `json:"value"`
	Reason  string  `json:"reason"`
}

type ingestBody struct {
	Days     []ingestDay      `json:"days" nullable:"false"`
	Rejected []ingestRejected `json:"rejected" nullable:"false" doc:"Values out of their bounds, or of a date after today in your time zone, left out; the others are stored."`
	Ignored  []string         `json:"ignored" nullable:"false" doc:"Metric names tdeemon does not read."`
	Workouts *haeWorkoutsBody `json:"workouts,omitempty" doc:"What a Health Auto Export payload's workouts did; absent when it had no list of them."`
}

type haeWorkoutsBody struct {
	Strength int      `json:"strength" doc:"The strength workouts in the payload up to today, which make their days training days."`
	Added    []string `json:"added" nullable:"false" doc:"The days that became days with a workout. A day keeps its workout once added: a workout deleted in Apple Health is the check-in's toggle to undo."`
	Future   []string `json:"future" nullable:"false" doc:"The days after the user's today with a strength workout, left out: a clock set wrong, or a time zone."`
	Skipped  []string `json:"skipped" nullable:"false" doc:"The names of the other kinds of workouts in the payload, which make no training day."`
}

type ingestOutput struct {
	Body ingestBody
}

// haePayloadSchema describes the body of ingest-hae for the document: huma
// gives a raw body a string's schema, and ParseHAE, not huma, reads it.
var haePayloadSchema = &huma.Schema{
	Type:        "object",
	Description: "Health Auto Export's REST API payload, as the app sends it.",
	Required:    []string{"data"},
	Properties: map[string]*huma.Schema{
		"data": {
			Type:        "object",
			Description: "The metrics, the workouts, or both: an automation of Health Auto Export sends one data type.",
			Properties: map[string]*huma.Schema{
				"workouts": {Type: "array", Description: "Workouts (v2): the strength ones make their days training days.", Items: &huma.Schema{
					Type: "object",
					Properties: map[string]*huma.Schema{
						"name":  {Type: "string", Description: "The kind of workout, such as Traditional Strength Training."},
						"start": {Type: "string", Description: "When it started, as 2006-01-02 15:04:05 -0700."},
					},
				}},
				"metrics": {Type: "array", Items: &huma.Schema{
					Type: "object",
					Properties: map[string]*huma.Schema{
						"name":  {Type: "string", Examples: []any{"step_count"}},
						"units": {Type: "string", Examples: []any{"count"}},
						"data": {Type: "array", Items: &huma.Schema{
							Type: "object", AdditionalProperties: true, Description: "A point: date, and qty or, for sleep, its totals.",
						}},
					},
				}},
			},
		},
	},
}

func (s *sessions) registerIngest(a huma.API) {
	huma.Register(a, huma.Operation{
		OperationID: "ingest-hae",
		Method:      http.MethodPost,
		Path:        Prefix + "/ingest/hae",
		Summary:     "Import a Health Auto Export payload",
		Description: "For the REST API automations of Health Auto Export, of Health Metrics and of Workouts: an API token with the ingest scope, not a web session. Each value belongs to the calendar date of its point; " +
			"the merge keeps a manual value, the day's first weighing and the larger of a running total (DESIGN.md §4). " +
			"A strength workout (" + strings.Join(ingest.HAEStrength(), ", ") + ") makes the day it starts in, by the user's day start hour, a training day unless the check-in's toggle says otherwise; " +
			"days of workouts from Health Auto Export are only added, never removed, as a payload cannot tell that it holds every workout of a day. tdeemon keeps the values it reads, not the body.",
		Tags:            []string{"ingest"},
		MaxBodyBytes:    maxIngestBytes,
		BodyReadTimeout: ingestReadTimeout,
		// HAE's JSON, which ParseHAE reads: huma would take a raw body for
		// a string.
		SkipValidateBody: true,
		Errors:           []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity},
		Metadata:         openToTokens(auth.ScopeIngest),
	}, s.ingestHAE)
	a.OpenAPI().Paths[Prefix+"/ingest/hae"].Post.RequestBody.Content["application/json"].Schema = haePayloadSchema
	huma.Register(a, huma.Operation{
		OperationID:  "ingest-metrics",
		Method:       http.MethodPost,
		Path:         Prefix + "/ingest/metrics",
		Summary:      "Import a day's metrics",
		Description:  "The simple form, for a script or another app: an API token with the ingest scope, not a web session. The values merge as an import, as Health Auto Export's do.",
		Tags:         []string{"ingest"},
		MaxBodyBytes: maxIngestBytes,
		Errors:       []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity},
		Metadata:     openToTokens(auth.ScopeIngest),
	}, s.ingestMetrics)
}

func dateStrings(ds []day.Date) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.String()
	}
	return out
}

func (s *sessions) ingestHAE(ctx context.Context, in *ingestHAEInput) (*ingestOutput, error) {
	p, err := requireToken(ctx)
	if err != nil {
		return nil, err
	}
	cal, err := day.NewCalendar(p.User.TimeZone, p.User.DayStartHour)
	if err != nil {
		return nil, internalError("importing the payload failed", fmt.Errorf("the user's calendar: %w", err))
	}
	w, err := ingest.ParseHAEWorkouts(in.RawBody, cal)
	if err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}
	r, err := ingest.ParseHAE(in.RawBody)
	switch {
	case errors.Is(err, ingest.ErrNotHAE) && w.Sent:
		// Workouts alone: the automation of Workouts.
		r = ingest.Result{}
	case errors.Is(err, ingest.ErrNotHAE):
		return nil, huma.Error422UnprocessableEntity(err.Error())
	case err != nil:
		return nil, huma.Error422UnprocessableEntity("the body does not read as a Health Auto Export payload")
	}
	out, err := s.store(ctx, p, "hae", r)
	if err != nil || !w.Sent {
		return out, err
	}
	if out.Body.Workouts, err = s.storeHAEWorkouts(ctx, p, w, cal.DayOf(s.now())); err != nil {
		return nil, err
	}
	return out, nil
}

// storeHAEWorkouts adds the days of w's strength workouts up to today to
// the user's days of workouts from Health Auto Export, and says what it
// added. It removes none: a payload cannot tell that it holds every
// workout of a day (an automation may send part of a day, or some kinds
// of workouts), and a day it lacks may still have one in Apple Health.
func (s *sessions) storeHAEWorkouts(ctx context.Context, p auth.Principal, w ingest.HAEWorkouts, today day.Date) (*haeWorkoutsBody, error) {
	strength := w.Strength.UpTo(today)
	added, err := s.db.AddWorkoutDays(ctx, p.User.ID, event.Actor{Kind: event.Import, Client: "hae"}, store.HAE, strength.Days, s.now())
	if err != nil {
		return nil, internalError("importing the workouts failed", err)
	}
	s.logger.InfoContext(ctx, "imported workouts from health auto export", "user_id", p.User.ID.String(), "strength", strength.Count,
		"added", len(added), "skipped", len(w.Skipped))
	return &haeWorkoutsBody{Strength: strength.Count, Added: dateStrings(added), Future: dateStrings(strength.Future), Skipped: w.Skipped}, nil
}

func (s *sessions) ingestMetrics(ctx context.Context, in *ingestMetricsInput) (*ingestOutput, error) {
	p, err := requireToken(ctx)
	if err != nil {
		return nil, err
	}
	r, err := ingest.FromDays(map[string]map[string]float64{in.Body.Date: in.Body.Metrics})
	if err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}
	return s.store(ctx, p, "ingest", r)
}

// store merges r's days into the user's, as an import of client's, and
// says what changed; a value of a date after the user's calendar date
// is rejected (calendarToday). A day stored before a later day fails stays: a retry
// of the whole body changes nothing more for it.
func (s *sessions) store(ctx context.Context, p auth.Principal, client string, r ingest.Result) (*ingestOutput, error) {
	a := event.Actor{Kind: event.Import, Client: client}
	today, err := calendarToday(p, s.now())
	if err != nil {
		return nil, internalError("importing the metrics failed", err)
	}
	r = r.UpTo(today)
	out := &ingestOutput{Body: ingestBody{Days: []ingestDay{}, Rejected: []ingestRejected{}, Ignored: []string{}}}
	for _, d := range r.Days {
		changes, err := s.db.MergeMetrics(ctx, p.User.ID, a, d.Date, d.Values, s.now())
		if err != nil {
			return nil, internalError("importing the metrics failed", fmt.Errorf("the day %s: %w", d.Date, err))
		}
		got := ingestDay{Date: d.Date.String(), Changed: []ingestChange{}}
		for _, c := range changes {
			ch := ingestChange{Metric: string(c.After.Kind), Value: c.After.Value}
			if c.Before != nil {
				ch.Before = &c.Before.Value
			}
			got.Changed = append(got.Changed, ch)
		}
		out.Body.Days = append(out.Body.Days, got)
	}
	for _, x := range r.Rejected {
		rej := ingestRejected{RawDate: x.RawDate, Metric: string(x.Kind), Value: x.Value, Reason: x.Reason}
		if !x.Date.IsZero() {
			rej.Date = x.Date.String()
		}
		out.Body.Rejected = append(out.Body.Rejected, rej)
	}
	out.Body.Ignored = append(out.Body.Ignored, r.Ignored...)
	s.logger.InfoContext(ctx, "imported metrics", "user_id", p.User.ID.String(), "client", client,
		"days", len(r.Days), "rejected", len(r.Rejected), "ignored", len(r.Ignored))
	return out, nil
}

// calendarToday is the user's calendar date at now, in their time zone,
// whatever hour their day starts: an import dates a value by its point's
// calendar date, so today's count in the hours before the day starts.
// A day after it is a clock or a time zone set wrong; a traveller ahead
// of home has those values when home's date reaches them, as a sync
// sends its range again.
func calendarToday(p auth.Principal, now time.Time) (day.Date, error) {
	c, err := day.NewCalendar(p.User.TimeZone, 0)
	if err != nil {
		return day.Date{}, fmt.Errorf("the user's calendar: %w", err)
	}
	return c.DayOf(now), nil
}
