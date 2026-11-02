package store

import (
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"
)

// 0033 makes recipe_shares food_shares, a group of any food whose seats
// are each the household's or an explicit share's: a share before it is
// an explicit one, its seats, versions and leaves as they were, and its
// members read as before. It adds a household's tables, and the events
// of joining one and leaving it.
func TestMigrationOfFoodShares(t *testing.T) {
	t.Parallel()

	db := openEmpty(t)
	if err := db.migrate(t.Context(), upTo(t, "0032_recipe_shares.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0032: %v", err)
	}
	must := func(q string, args ...any) {
		t.Helper()
		if _, err := db.write.ExecContext(t.Context(), q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	ana := uuid.MustParse("0199f0a0-0000-7000-8000-000000000001")
	ivo := uuid.MustParse("0199f0a0-0000-7000-8000-000000000002")
	eva := uuid.MustParse("0199f0a0-0000-7000-8000-000000000003")
	for _, u := range []struct {
		id    uuid.UUID
		email string
	}{{ana, "ana@example.com"}, {ivo, "ivo@example.com"}, {eva, "eva@example.com"}} {
		must(`INSERT INTO users (id, email, name, role, locale, timezone, day_start_hour, created_at, updated_at)
			VALUES (?, ?, 'X', 'user', 'en', 'UTC', 4, '2026-11-01T08:00:00Z', '2026-11-01T08:00:00Z')`, u.id.String(), u.email)
	}
	now := formatTime(time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC))
	stew := map[uuid.UUID]uuid.UUID{ana: uuid.NewV7(), ivo: uuid.NewV7(), eva: uuid.NewV7()}
	for u, id := range stew {
		must(`INSERT INTO foods (id, user_id, name, name_key, basis, kcal, protein, carbs, fat, source, created_at, updated_at)
			VALUES (?, ?, 'Stew', 'stew', '100g', 90, 6, 8, 3, 'recipe', ?, ?)`, id.String(), u.String(), now, now)
		must(`INSERT INTO recipes (food_id, user_id, yield_g) VALUES (?, ?, 800)`, id.String(), u.String())
	}
	// Ana shared her stew with Ivo and Eva, and Eva left it: the group's
	// version 3, her row at the version she left it at.
	share := uuid.NewV7().String()
	must(`INSERT INTO recipe_shares (share_id, seat, user_id, food_id, version, left_at) VALUES
		(?1, 1, ?2, ?3, 3, NULL), (?1, 2, ?4, ?5, 3, NULL), (?1, 3, ?6, ?7, 2, ?8)`,
		share, ana.String(), stew[ana].String(), ivo.String(), stew[ivo].String(), eva.String(), stew[eva].String(), now)
	must(`INSERT INTO events (id, user_id, at, actor, client, operation, entity) VALUES (?, ?, ?, 'user', 'web', 'recipe.share', ?)`,
		uuid.NewV7().String(), ana.String(), now, stew[ana].String())

	if err := db.migrate(t.Context(), upTo(t, "0033_food_shares.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0033: %v", err)
	}
	carriedShares(t, db, share, []string{
		ana.String() + " " + stew[ana].String() + "  3",
		ivo.String() + " " + stew[ivo].String() + "  3",
		eva.String() + " " + stew[eva].String() + " " + now + " 2",
	})
	if members, err := db.RecipeMembers(t.Context(), ivo, stew[ivo]); err != nil || len(members) != 2 ||
		members[0].ID != ivo || members[1].ID != ana || members[1].Seat != 1 {
		t.Errorf("Ivo's members after 0033 = %+v, %v; want Ivo, then Ana in seat 1", members, err)
	}
	if _, err := db.RecipeMembers(t.Context(), eva, stew[eva]); err == nil {
		t.Errorf("Eva's members after 0033: no error; want the stew she left gone from her picks")
	}

	// A plain food may be in a group now, as a household's seat.
	bread := uuid.NewV7()
	must(`INSERT INTO foods (id, user_id, name, name_key, basis, kcal, protein, carbs, fat, source, created_at, updated_at)
		VALUES (?, ?, 'Bread', 'bread', '100g', 250, 9, 48, 3, 'label', ?, ?)`, bread.String(), ana.String(), now, now)
	must(`INSERT INTO food_shares (share_id, seat, user_id, food_id, household) VALUES (?, 1, ?, ?, 1)`,
		uuid.NewV7().String(), ana.String(), bread.String())
	for _, op := range []string{"household.join", "household.leave"} {
		must(`INSERT INTO events (id, user_id, at, actor, client, operation, entity) VALUES (?, ?, ?, 'user', 'web', ?, 'household')`,
			uuid.NewV7().String(), ana.String(), now, op)
	}
	householdTables(t, db, ana, ivo, eva, now)
}

// carriedShares holds food_shares to be recipe_shares as it was, by
// seat (each a "user food left_at version"), every row of share and an
// explicit share's, and recipe_shares to be gone.
func carriedShares(t *testing.T, db *DB, share string, want []string) {
	t.Helper()
	rows, err := db.write.QueryContext(t.Context(),
		"SELECT share_id, seat, user_id, food_id, version, coalesce(left_at, ''), household FROM food_shares ORDER BY seat")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var got []string
	for rows.Next() {
		var s, u, f, left string
		var seat, version, household int
		if err := rows.Scan(&s, &seat, &u, &f, &version, &left, &household); err != nil {
			t.Fatal(err)
		}
		if s != share || household != 0 {
			t.Errorf("seat %d: share %s, household %d; want %s, an explicit share's", seat, s, household, share)
		}
		got = append(got, fmt.Sprintf("%s %s %s %d", u, f, left, version))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("food_shares = %q; want %q", got, want)
	}
	var gone bool
	if err := db.write.QueryRowContext(t.Context(),
		"SELECT NOT EXISTS (SELECT 1 FROM sqlite_schema WHERE name = 'recipe_shares')").Scan(&gone); err != nil || !gone {
		t.Errorf("recipe_shares gone = %v, %v; want it dropped", gone, err)
	}
}

// householdTables holds the household's tables to their rules: a user
// in one household at most, one invite waiting per inviter and invited
// user, none of oneself, and a user's deletion taking their membership
// and every invite of or to them.
func householdTables(t *testing.T, db *DB, ana, ivo, eva uuid.UUID, now string) {
	t.Helper()
	exec := func(q string, args ...any) error {
		_, err := db.write.ExecContext(t.Context(), q, args...)
		return err
	}
	home, other := uuid.NewV7().String(), uuid.NewV7().String()
	for _, h := range []string{home, other} {
		if err := exec("INSERT INTO households (id, created_at) VALUES (?, ?)", h, now); err != nil {
			t.Fatal(err)
		}
	}
	join := "INSERT INTO household_members (user_id, household_id, joined_at) VALUES (?, ?, ?)"
	for _, u := range []uuid.UUID{ana, ivo} {
		if err := exec(join, u.String(), home, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := exec(join, ana.String(), other, now); err == nil {
		t.Error("Ana in a second household: no error; want one household a user")
	}
	invite := "INSERT INTO household_invites (id, user_id, invited_id, created_at, expires_at) VALUES (?, ?, ?, ?, ?)"
	if err := exec(invite, uuid.NewV7().String(), ana.String(), eva.String(), now, now); err != nil {
		t.Fatal(err)
	}
	if err := exec(invite, uuid.NewV7().String(), ana.String(), eva.String(), now, now); err == nil {
		t.Error("a second invite of Ana's to Eva: no error; want one waiting a pair")
	}
	if err := exec(invite, uuid.NewV7().String(), eva.String(), eva.String(), now, now); err == nil {
		t.Error("Eva inviting herself: no error")
	}
	if err := exec(invite, uuid.NewV7().String(), eva.String(), ivo.String(), now, now); err != nil {
		t.Fatal(err)
	}
	if err := exec("DELETE FROM users WHERE id = ?", eva.String()); err != nil {
		t.Fatal(err)
	}
	if err := exec("DELETE FROM users WHERE id = ?", ana.String()); err != nil {
		t.Fatal(err)
	}
	var invites, members int
	if err := db.write.QueryRowContext(t.Context(),
		"SELECT (SELECT count(*) FROM household_invites), (SELECT count(*) FROM household_members)").Scan(&invites, &members); err != nil {
		t.Fatal(err)
	}
	if invites != 0 || members != 1 {
		t.Errorf("after Eva's and Ana's deletion: %d invites, %d members; want none, and Ivo", invites, members)
	}
}
