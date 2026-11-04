package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/tdeemon/tdeemon/internal/auth"
	"github.com/tdeemon/tdeemon/internal/domain/user"
	"github.com/tdeemon/tdeemon/internal/store"
)

// user demo adds a user with demo data and prints their password once.
func TestUserDemo(t *testing.T) {
	t.Parallel()

	env, dbPath := instance(t, nil)
	var out bytes.Buffer
	args := []string{"user", "demo", "--email", "Demo@Example.com", "--name", "Demo", "--days", "40", "--seed", "3", "--locale", "hr", "--timezone", "Europe/Zagreb"}
	if err := run(t.Context(), args, env, noInput(), &out, io.Discard); err != nil {
		t.Fatalf("user demo: %v", err)
	}
	u := checkCreated(t, dbPath, "demo@example.com", out.String())
	if u.Name != "Demo" || u.Locale != "hr" || u.TimeZone != "Europe/Zagreb" {
		t.Errorf("stored user = %+v, want Demo in Croatian in Europe/Zagreb", u)
	}

	db, err := store.Open(t.Context(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if phases, err := db.Phases(t.Context(), u.ID); err != nil || len(phases) != 1 {
		t.Errorf("phases %+v (%v), want the cut", phases, err)
	}
	if meals, err := db.RecentMeals(t.Context(), u.ID, 50); err != nil || len(meals) == 0 {
		t.Errorf("recent meals %d (%v), want some", len(meals), err)
	}

	// The address is taken now: a second demo user of it is refused, and
	// the first keeps their data.
	err = run(t.Context(), []string{"user", "demo", "--email", "demo@example.com"}, env, noInput(), io.Discard, io.Discard)
	if !errors.Is(err, store.ErrEmailTaken) {
		t.Errorf("a taken address: %v, want ErrEmailTaken", err)
	}
	if n := userCount(t, dbPath); n != 1 {
		t.Errorf("%d users, want 1", n)
	}
	if phases, err := db.Phases(t.Context(), u.ID); err != nil || len(phases) != 1 {
		t.Errorf("phases after the refusal %+v (%v), want the cut", phases, err)
	}
}

// checkCreated checks that out reports the demo user of email created,
// with the password they were stored with, and returns them.
func checkCreated(t *testing.T, dbPath, email, out string) user.User {
	t.Helper()

	u, hash := stored(t, dbPath, email)
	if !strings.HasPrefix(out, "Created user "+email+" (id "+u.ID.String()+") with demo data from ") {
		t.Errorf("output %q does not report the new user", out)
	}
	m := printedPassword.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("output %q prints no password", out)
	}
	if ok, err := auth.VerifyPassword(t.Context(), m[1], hash); err != nil || !ok {
		t.Errorf("the printed password does not match the stored hash (ok %v, err %v)", ok, err)
	}
	return u
}

// user demo refuses a command line it does not take, with the usage.
func TestUserDemoUsage(t *testing.T) {
	t.Parallel()

	env, dbPath := instance(t, nil)
	for name, args := range map[string][]string{
		"no email":       {"--days", "40"},
		"too few days":   {"--email", "demo@example.com", "--days", "29"},
		"too many days":  {"--email", "demo@example.com", "--days", "366"},
		"days no number": {"--email", "demo@example.com", "--days", "many"},
		"a bad seed":     {"--email", "demo@example.com", "--seed", "-1"},
		"an argument":    {"--email", "demo@example.com", "more"},
		"a bad flag":     {"--mail", "demo@example.com"},
	} {
		err := run(t.Context(), append([]string{"user", "demo"}, args...), env, noInput(), io.Discard, io.Discard)
		if !errors.Is(err, errUsage) {
			t.Errorf("%s: %v, want a usage error", name, err)
		}
		if code := exitCode(err, io.Discard); code != 2 {
			t.Errorf("%s: exit %d, want 2", name, code)
		}
	}
	if n := userCount(t, dbPath); n != 0 {
		t.Errorf("%d users after the refusals, want none", n)
	}
	var out bytes.Buffer
	if err := run(t.Context(), []string{"user", "demo", "--help"}, env, noInput(), &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Flags of user demo:") {
		t.Errorf("help %q does not tell the flags of user demo", out.String())
	}
	if err := run(t.Context(), []string{"user", "demo", "--email", "not an address"}, env, noInput(), io.Discard, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "invalid user") {
		t.Errorf("a bad address: %v, want an invalid user", err)
	}
}
