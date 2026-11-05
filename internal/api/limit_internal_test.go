package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"testing"
	"time"
	"uuid"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tdeemon/tdeemon/internal/auth"
	"github.com/tdeemon/tdeemon/internal/domain/user"
	"github.com/tdeemon/tdeemon/internal/store/storetest"
)

func TestLimiter(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)
	l := newLimiter(3, time.Minute, func() time.Time { return now })
	var first reservation
	for i := range 3 {
		r, _, ok := l.take("a")
		if !ok {
			t.Fatalf("attempt %d refused, want three", i+1)
		}
		if i == 0 {
			first = r
		}
		now = now.Add(10 * time.Second)
	}
	if _, wait, ok := l.take("a"); ok || wait != 30*time.Second {
		t.Errorf("a fourth attempt: allowed %v, wait %v; want refused for the 30s left of the window", ok, wait)
	}
	if _, _, ok := l.take("b"); !ok {
		t.Error("another key is refused")
	}
	// An attempt given back makes room for one more.
	l.release(first)
	if _, _, ok := l.take("a"); !ok {
		t.Error("refused after an attempt was given back")
	}
	now = now.Add(30 * time.Second)
	if _, _, ok := l.take("a"); !ok {
		t.Error("still refused once the window has passed")
	}
	for range 3 {
		l.take("c")
	}
	l.reset("c")
	if _, _, ok := l.take("c"); !ok {
		t.Error("refused after a reset")
	}
}

// An attempt given back once its window has passed does not count against
// the next window; a key with no attempts left goes from the map.
func TestLimiterReleases(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)
	l := newLimiter(2, time.Minute, func() time.Time { return now })
	old, _, _ := l.take("a")
	now = now.Add(time.Minute) // a new window
	l.take("a")
	l.take("a")
	l.release(old)
	if _, _, ok := l.take("a"); ok {
		t.Error("an attempt from the window before made room in this one")
	}
	r, _, _ := l.take("b")
	l.release(r)
	if _, ok := l.keys["b"]; ok {
		t.Error("a key with no attempts left stayed in the map")
	}
}

// Past maxKeys, expired keys make room, and a key that still does not fit
// is not counted. Attempts given back take no room.
func TestLimiterBoundsItsKeys(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)
	l := newLimiter(1, time.Minute, func() time.Time { return now })
	l.maxKeys = 2
	for i := range 100 {
		r, _, _ := l.take(fmt.Sprint("given back ", i))
		l.release(r)
	}
	l.take("old")
	now = now.Add(time.Minute)
	l.take("a")
	l.take("b") // "old" expired: swept
	if _, _, ok := l.take("b"); ok {
		t.Error("b was not counted, though an expired key made room")
	}
	l.take("c") // full of live keys
	if _, _, ok := l.take("c"); !ok {
		t.Error("c was counted past the bound")
	}
	if len(l.keys) > 2 {
		t.Errorf("%d keys, want at most 2", len(l.keys))
	}
}

func TestRetryAfter(t *testing.T) {
	t.Parallel()

	for wait, want := range map[time.Duration]string{time.Millisecond: "1", 90 * time.Second: "90", 1500 * time.Millisecond: "2"} {
		t.Run(want, func(t *testing.T) {
			t.Parallel()
			if got := retryAfter(wait).Get("Retry-After"); got != want {
				t.Errorf("retryAfter(%v) = %q, want %q", wait, got, want)
			}
		})
	}
}

// After a sweep, oldest is the oldest key left, so the next key to expire
// makes room when its time comes, not a window later.
func TestLimiterSweepFindsTheOldestLeft(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)
	now := start
	l := newLimiter(1, time.Minute, func() time.Time { return now })
	l.maxKeys = 2
	l.take("x")
	now = start.Add(30 * time.Second)
	l.take("y")
	now = start.Add(time.Minute)
	l.take("z") // x has expired: swept
	now = start.Add(90 * time.Second)
	l.take("w") // y has expired: room for w
	if _, _, ok := l.take("w"); ok {
		t.Error("w was not counted, though y had expired")
	}
}

// A full limiter whose keys expire one by one is swept at most maxSweeps
// times a window, not for every new key.
func TestLimiterSweepsAFullMapRarely(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)
	now := start
	l := newLimiter(1, 64*time.Minute, func() time.Time { return now })
	l.maxKeys = 100
	for i := range 100 {
		now = start.Add(time.Duration(i) * time.Second)
		l.take(fmt.Sprint("old ", i))
	}
	// The old keys expire a second apart; new ones come every second.
	for i := range 100 {
		now = start.Add(64*time.Minute + time.Duration(i)*time.Second)
		l.take(fmt.Sprint("new ", i))
	}
	if l.sweeps != 2 {
		t.Errorf("%d sweeps in 100 seconds, want 2 (one a minute)", l.sweeps)
	}
	// The second sweep, a minute after the first, made room for it.
	if _, _, ok := l.take("new 60"); ok {
		t.Error("new 60 was not counted, though a sweep was due")
	}
}

func TestLogLevel(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		err  error
		want slog.Level
	}{
		{context.Canceled, slog.LevelDebug},
		{fmt.Errorf("query: %w", context.DeadlineExceeded), slog.LevelDebug},
		{auth.ErrBusy, slog.LevelWarn},
		{errors.New("disk I/O error"), slog.LevelError},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			t.Parallel()
			if got := logLevel(tc.err); got != tc.want {
				t.Errorf("logLevel = %v, want %v", got, tc.want)
			}
		})
	}
}

// newLimitSessions is the sessions of an instance with Ana, whose
// password is "correct horse battery staple", checked without a hash: a
// test of the limits signs in hundreds of times, and argon2id under the
// race detector takes a quarter of a second each.
func newLimitSessions(t *testing.T) *sessions {
	t.Helper()

	db := storetest.Open(t)
	now := time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC)
	u := user.User{
		ID: uuid.NewV7(), Email: "ana@example.com", Name: "Ana", Role: user.RoleUser,
		Locale: "en", TimeZone: "UTC", DayStartHour: 4, CreatedAt: now, UpdatedAt: now,
	}
	hash, err := auth.HashPassword(t.Context(), "correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := db.CreateUser(t.Context(), u, hash); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	s := newTestSessions(db, now)
	s.verifyLogin = func(_ context.Context, password, encoded string) (bool, error) {
		return encoded == hash && password == "correct horse battery staple", nil
	}
	return s
}

// loginFrom signs in as email with password from addr, and returns the
// answer's status.
func loginFrom(t *testing.T, s *sessions, addr, email, password string) int {
	t.Helper()

	ctx := context.WithValue(t.Context(), clientAddrKey{}, netip.MustParseAddr(addr))
	in := &loginInput{}
	in.Body.Email, in.Body.Password = email, password
	if _, err := s.login(ctx, in); err != nil {
		se, ok := errors.AsType[huma.StatusError](err)
		if !ok {
			t.Fatalf("login: %v", err)
		}
		return se.GetStatus()
	}
	return http.StatusOK
}

// A hundred failed sign-ins for one account, from any addresses, and the
// account gets 429 everywhere until the window passes; a success before
// that starts its count over. An address nobody has counts the same, so
// the answer tells nothing about which accounts exist.
func TestFailedSignInsForOneAccount(t *testing.T) {
	t.Parallel()

	const password, wrong = "correct horse battery staple", "wrong password, surely"
	s := newLimitSessions(t)
	addr := func(i int) string { return fmt.Sprintf("203.0.113.%d", i) }
	for _, email := range []string{"ana@example.com", "nobody@example.com"} {
		for i := range 99 {
			if code := loginFrom(t, s, addr(i), email, wrong); code != http.StatusUnauthorized {
				t.Fatalf("%s, failure %d: %d, want 401", email, i+1, code)
			}
		}
	}
	// Ana's success starts her count over; nobody's goes on.
	if code := loginFrom(t, s, addr(200), "ana@example.com", password); code != http.StatusOK {
		t.Fatalf("ana after 99 failures: %d, want 200", code)
	}
	if code := loginFrom(t, s, addr(201), "nobody@example.com", wrong); code != http.StatusUnauthorized {
		t.Fatalf("nobody's 100th failure: %d, want 401", code)
	}
	if code := loginFrom(t, s, addr(202), "nobody@example.com", wrong); code != http.StatusTooManyRequests {
		t.Errorf("nobody after 100 failures: %d, want 429", code)
	}
	for i := range 99 {
		if code := loginFrom(t, s, fmt.Sprintf("192.0.2.%d", i), "ana@example.com", wrong); code != http.StatusUnauthorized {
			t.Fatalf("ana, failure %d after the success: %d, want 401", i+1, code)
		}
	}
	if code := loginFrom(t, s, addr(220), "ana@example.com", password); code != http.StatusOK {
		t.Errorf("ana after 99 new failures: %d, want 200 (the success had started over)", code)
	}
}

// Someone who knows a user's address cannot keep them from signing in
// from an address of their own, nor from a few: each is blocked after
// ten failures, long before the account is.
func TestFailedSignInsDoNotLockAnAccountOut(t *testing.T) {
	t.Parallel()

	s := newLimitSessions(t)
	for a := range 5 {
		attacker := fmt.Sprintf("198.51.100.%d", 40+a)
		for i := range 12 {
			want := http.StatusUnauthorized
			if i >= 10 {
				want = http.StatusTooManyRequests
			}
			if code := loginFrom(t, s, attacker, "ana@example.com", "wrong password, surely"); code != want {
				t.Fatalf("attacker %d, try %d: %d, want %d", a, i+1, code, want)
			}
		}
	}
	if code := loginFrom(t, s, "203.0.113.50", "ana@example.com", "correct horse battery staple"); code != http.StatusOK {
		t.Errorf("ana from her own address after 50 failures from five others: %d, want 200", code)
	}
}
