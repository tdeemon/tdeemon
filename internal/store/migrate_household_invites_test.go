package store

import (
	"testing"
	"time"
	"uuid"
)

// 0034 makes an invite into a household the address's: each waiting
// invite names the address of the user it invited.
func TestMigrationOfHouseholdInvites(t *testing.T) {
	t.Parallel()

	db := openEmpty(t)
	if err := db.migrate(t.Context(), upTo(t, "0033_food_shares.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0033: %v", err)
	}
	must := func(q string, args ...any) {
		t.Helper()
		if _, err := db.write.ExecContext(t.Context(), q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	ana := uuid.MustParse("0199f0a0-0000-7000-8000-000000000001")
	ivo := uuid.MustParse("0199f0a0-0000-7000-8000-000000000002")
	for _, u := range []struct {
		id    uuid.UUID
		email string
	}{{ana, "ana@example.com"}, {ivo, "ivo@example.com"}} {
		must(`INSERT INTO users (id, email, name, role, locale, timezone, day_start_hour, created_at, updated_at)
			VALUES (?, ?, 'X', 'user', 'en', 'UTC', 4, '2026-11-01T08:00:00Z', '2026-11-01T08:00:00Z')`, u.id.String(), u.email)
	}
	now := formatTime(time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC))
	invite := uuid.NewV7().String()
	must("INSERT INTO household_invites (id, user_id, invited_id, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		invite, ana.String(), ivo.String(), now, now)

	if err := db.migrate(t.Context(), upTo(t, "9999"), discard()); err != nil {
		t.Fatalf("migrate from 0033: %v", err)
	}
	var id, by, email string
	if err := db.write.QueryRowContext(t.Context(), "SELECT id, user_id, email FROM household_invites").Scan(&id, &by, &email); err != nil {
		t.Fatal(err)
	}
	if id != invite || by != ana.String() || email != "ivo@example.com" {
		t.Errorf("the invite = %s by %s of %s; want %s by Ana of Ivo's address", id, by, email, invite)
	}
}
