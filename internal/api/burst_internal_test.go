package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/tdeemon/tdeemon/internal/store/storetest"
)

// Sign-ins sent all at once cannot slip past the limit: the attempts
// still being checked count, so of a burst of wrong passwords from one
// address at most ten are checked, and for one account a hundred.
//
// The check holds every sign-in that reaches it until the whole burst is
// in, so all of them are in flight at once, whatever the machine's load.
// It takes no hashing slot: real hashes of a burst this size queue past
// the 10 seconds a sign-in waits on a loaded CI runner, and make the
// other tests' sign-ins wait too (issue 284).
func TestABurstOfSignInsMeetsTheLimit(t *testing.T) {
	t.Parallel()

	for name, c := range map[string]struct {
		addr       func(i int) string
		sent, most int
	}{
		"from one address": {func(int) string { return "198.51.100.20" }, 40, 10},
		"for one account":  {func(i int) string { return fmt.Sprintf("203.%d.113.%d", i/10, i%10) }, 140, 100},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := newTestSessions(storetest.Open(t), time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC))
			var checked, refused atomic.Int64
			release := make(chan struct{})
			s.verifyLogin = func(ctx context.Context, _, _ string) (bool, error) {
				checked.Add(1)
				select {
				case <-release:
				case <-ctx.Done():
				}
				return false, nil
			}

			// One after another: each sign-in is held in the check, or
			// refused, before the next one comes. Ones still being checked
			// must count, or every one of them gets checked.
			var wg sync.WaitGroup
			for i := range c.sent {
				ctx := context.WithValue(t.Context(), clientAddrKey{}, netip.MustParseAddr(c.addr(i)))
				wg.Go(func() {
					in := &loginInput{}
					in.Body.Email, in.Body.Password = "ana@example.com", "wrong password, surely"
					_, err := s.login(ctx, in)
					se, ok := errors.AsType[huma.StatusError](err)
					switch {
					case ok && se.GetStatus() == http.StatusTooManyRequests:
						refused.Add(1)
					case ok && se.GetStatus() == http.StatusUnauthorized:
					default:
						t.Errorf("a sign-in answered %v, want 401 or 429", err)
					}
				})
				deadline := time.Now().Add(30 * time.Second)
				for checked.Load()+refused.Load() < int64(i+1) && time.Now().Before(deadline) {
					time.Sleep(100 * time.Microsecond)
				}
			}
			inFlight := checked.Load()
			close(release)
			wg.Wait()
			if inFlight+refused.Load() != int64(c.sent) {
				t.Fatalf("%d checked and %d refused of %d sent: the burst never all arrived", inFlight, refused.Load(), c.sent)
			}
			if inFlight > int64(c.most) {
				t.Errorf("%d of %d sign-ins were checked at once, want at most %d", inFlight, c.sent, c.most)
			}
			if inFlight == 0 {
				t.Error("no sign-in was checked")
			}
		})
	}
}
