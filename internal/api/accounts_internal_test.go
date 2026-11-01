package api

import (
	"testing"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/auth"
	"github.com/zarnautovic/tdeemon/internal/domain/user"
	"github.com/zarnautovic/tdeemon/internal/store/storetest"
)

// An account that another delete took after the session was checked is
// gone as asked: the delete answers as if it had taken it, not 404.
func TestDeleteAccountAlreadyGone(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	s := newTestSessions(storetest.Open(t), now)
	gone := user.User{ID: uuid.NewV7(), Email: "ivo@example.com", Name: "Ivo", Role: user.RoleUser}
	ctx := auth.WithPrincipal(t.Context(), auth.Principal{User: gone, Via: auth.ViaSession})
	in := &deleteAccountInput{}
	in.Body.Email = gone.Email
	out, err := s.deleteAccount(ctx, in)
	if err != nil || out.SetCookie.MaxAge >= 0 {
		t.Errorf("deleteAccount = %+v, %v; want the cookie cleared", out, err)
	}
}
