package store_test

import (
	"database/sql"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/tdeemon/tdeemon/internal/auth"
	"github.com/tdeemon/tdeemon/internal/domain/user"
	"github.com/tdeemon/tdeemon/internal/store"
	"github.com/tdeemon/tdeemon/internal/store/storetest"
)

// Only an active admin's invite is made, and works: an invite of an
// admin made a user since, as a request already under way may make it,
// makes no account.
func TestInviteNeedsAnActiveAdmin(t *testing.T) {
	t.Parallel()

	db := storetest.Open(t)
	ana, ivo := newUser("ana@example.com"), newUser("ivo@example.com")
	ana.Role, ivo.Role = user.RoleAdmin, user.RoleAdmin
	for _, u := range []user.User{ana, ivo} {
		if err := db.CreateUser(t.Context(), u, ""); err != nil {
			t.Fatal(err)
		}
	}
	inv := func(by uuid.UUID, email string) error {
		i := store.Invite{ID: uuid.NewV7(), By: by, Email: email, Role: user.RoleAdmin, CreatedAt: t0, ExpiresAt: t0.Add(time.Hour)}
		_, hash := auth.NewSessionToken()
		return db.CreateInvite(t.Context(), i, hash)
	}
	if err := inv(ivo.ID, "zoe@example.com"); err != nil {
		t.Fatal(err)
	}
	// Ivo is made a user; a request under way still writes an invite of
	// his, straight in the table as the race would leave it.
	role := user.RoleUser
	if _, err := db.ChangeUser(t.Context(), ivo.ID, store.UserChange{Role: &role}, t0); err != nil {
		t.Fatal(err)
	}
	if err := inv(ivo.ID, "mia@example.com"); !errors.Is(err, store.ErrNotAdmin) {
		t.Errorf("an invite of a user: %v, want ErrNotAdmin", err)
	}
	_, late := auth.NewSessionToken()
	err := db.WriteTx(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(), "INSERT INTO invites (id, token_hash, user_id, email, role, created_at, expires_at) "+
			"VALUES (?, ?, ?, 'eve@example.com', 'admin', ?, ?)", uuid.NewV7().String(), late, ivo.ID.String(),
			t0.Format(time.RFC3339), t0.Add(time.Hour).Format(time.RFC3339))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.InviteByToken(t.Context(), late, t0); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("a late invite of a user: %v, want ErrNotFound", err)
	}
	eve := newUser("eve@example.com")
	s := store.Session{ID: auth.SessionID("eve's"), UserID: eve.ID, CreatedAt: t0, LastSeenAt: t0, ExpiresAt: t0.Add(time.Hour)}
	if _, err := db.AcceptInvite(t.Context(), late, store.Acceptance{User: eve, Session: s}, t0); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("accept a late invite of a user: %v, want ErrNotFound", err)
	}
}
