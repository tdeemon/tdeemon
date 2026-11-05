package api_test

import (
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tdeemon/tdeemon/internal/auth"
)

// from is in.do's header map for a request from addr (host:port).
func from(addr string, header map[string]string) map[string]string {
	h := map[string]string{"RemoteAddr": addr}
	maps.Copy(h, header)
	return h
}

func (in *instance) loginFrom(t *testing.T, addr, email, pw string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, pw)
	return in.do(t, http.MethodPost, "/api/v1/auth/login", body, nil, from(addr, nil))
}

// Ten failed sign-ins from one address in 15 minutes, and that address
// gets 429 with Retry-After until they have passed, the right password
// included; another address goes on. A success from there in between
// does not start the address's count over: signing in to one's own
// account must not wipe the failures against others.
func TestFailedSignInsFromOneAddress(t *testing.T) {
	t.Parallel()

	in := newInstance(t, "http://localhost", passwordHash(t))
	const attacker = "198.51.100.7:4000"
	for i := range 10 {
		if i == 9 {
			if res := in.loginFrom(t, attacker, "ana@example.com", password); res.Code != http.StatusOK {
				t.Fatalf("a success after nine failures: %d, want 200", res.Code)
			}
		}
		// Another account each time, so only the address adds up.
		if res := in.loginFrom(t, attacker, fmt.Sprintf("guess%d@example.com", i), "wrong password, surely"); res.Code != http.StatusUnauthorized {
			t.Fatalf("failure %d: %d, want 401", i+1, res.Code)
		}
	}
	res := in.loginFrom(t, attacker, "ana@example.com", password)
	if res.Code != http.StatusTooManyRequests || res.Header().Get("Retry-After") != "900" {
		t.Errorf("the right password after ten failures: %d, Retry-After %q; want 429 and 900", res.Code, res.Header().Get("Retry-After"))
	}
	if res := in.loginFrom(t, "198.51.100.8:4000", "ana@example.com", password); res.Code != http.StatusOK {
		t.Errorf("from another address: %d, want 200", res.Code)
	}
	in.clock.Set(in.clock.Now().Add(15 * time.Minute))
	if res := in.loginFrom(t, attacker, "ana@example.com", password); res.Code != http.StatusOK {
		t.Errorf("after 15 minutes: %d, want 200", res.Code)
	}
}

// Fifty bad API tokens from one address in 15 minutes, and a bad token
// from there gets 429 until they have passed. A good token goes on, from
// there too (a device behind the same NAT), and does not start the count
// over; so do the session cookie and other addresses.
func TestBadTokensFromOneAddress(t *testing.T) {
	t.Parallel()

	in, session := signedIn(t)
	tok, _ := in.createToken(t, session, `{"name":"iPhone","scopes":["ingest"]}`)
	const attacker = "198.51.100.9:6000"
	for i := range 50 {
		bad := bearer(auth.TokenPrefix + strings.Repeat("A", 42) + fmt.Sprint(i%10))
		if rec := in.do(t, http.MethodGet, "/api/v1/auth/whoami", "", nil, from(attacker, bad)); rec.Code != http.StatusUnauthorized {
			t.Fatalf("bad token %d: %d, want 401", i+1, rec.Code)
		}
	}
	bad := bearer(auth.TokenPrefix + strings.Repeat("B", 43))
	rec := in.do(t, http.MethodGet, "/api/v1/auth/whoami", "", nil, from(attacker, bad))
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") != "900" {
		t.Errorf("a bad token after fifty: %d, Retry-After %q; want 429 and 900", rec.Code, rec.Header().Get("Retry-After"))
	}
	if rec := in.do(t, http.MethodGet, "/api/v1/auth/whoami", "", nil, from(attacker, bearer(tok.Token))); rec.Code != http.StatusOK {
		t.Errorf("the good token from there: %d, want 200", rec.Code)
	}
	if rec := in.do(t, http.MethodGet, "/api/v1/auth/whoami", "", nil, from(attacker, bearer(""))); rec.Code != http.StatusTooManyRequests {
		t.Errorf("an empty token after the good one: %d, want 429", rec.Code)
	}
	if rec := in.do(t, http.MethodGet, "/api/v1/auth/whoami", "", []*http.Cookie{session}, from(attacker, nil)); rec.Code != http.StatusOK {
		t.Errorf("the session from there: %d, want 200", rec.Code)
	}
	if rec := in.do(t, http.MethodGet, "/api/v1/auth/whoami", "", nil, from("198.51.100.10:6000", bearer(tok.Token))); rec.Code != http.StatusOK {
		t.Errorf("the good token from another address: %d, want 200", rec.Code)
	}
	in.clock.Set(in.clock.Now().Add(15 * time.Minute))
	if rec := in.do(t, http.MethodGet, "/api/v1/auth/whoami", "", nil, from(attacker, bad)); rec.Code != http.StatusUnauthorized {
		t.Errorf("a bad token after 15 minutes: %d, want 401", rec.Code)
	}
}

// Only failures add up: sign-ins that succeed, and requests with a good
// token, give their attempt back.
func TestSuccessesDoNotUseUpTheLimit(t *testing.T) {
	t.Parallel()

	in, session := signedIn(t)
	tok, _ := in.createToken(t, session, `{"name":"iPhone","scopes":["ingest"]}`)
	const addr = "198.51.100.30:8000"
	for i := range 12 {
		if res := in.loginFrom(t, addr, "ana@example.com", password); res.Code != http.StatusOK {
			t.Fatalf("sign-in %d: %d, want 200", i+1, res.Code)
		}
	}
	for i := range 60 {
		if rec := in.do(t, http.MethodGet, "/api/v1/auth/whoami", "", nil, from(addr, bearer(tok.Token))); rec.Code != http.StatusOK {
			t.Fatalf("request %d with a good token: %d, want 200", i+1, rec.Code)
		}
	}
}
