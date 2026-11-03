package store_test

import (
	"errors"
	"reflect"
	"testing"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/store"
)

// invite is inviter's invite of invited into their household, at t0.
func invite(t *testing.T, db *store.DB, inviter, invited uuid.UUID) store.HouseholdInvite {
	t.Helper()
	inv, err := db.InviteToHousehold(t.Context(), inviter, invited, t0)
	if err != nil {
		t.Fatalf("InviteToHousehold: %v", err)
	}
	return inv
}

// join is invited's accept of inviter's invite into their household,
// after its preview, whose report the accept's must be; the household
// keeps one kitchen then.
func join(t *testing.T, db *store.DB, inviter, invited uuid.UUID) store.MergeReport {
	t.Helper()
	inv := invite(t, db, inviter, invited)
	preview, err := db.PreviewHouseholdInvite(t.Context(), invited, inv.ID, t0)
	if err != nil {
		t.Fatalf("PreviewHouseholdInvite: %v", err)
	}
	got, _, err := db.AcceptHouseholdInvite(t.Context(), invited, testActor, inv.ID, preview.Fingerprint, t0)
	if err != nil {
		t.Fatalf("AcceptHouseholdInvite: %v", err)
	}
	if !reflect.DeepEqual(got, preview.Report) {
		t.Errorf("the accept did %+v, its preview showed %+v", got, preview.Report)
	}
	householdHolds(t, db, membersOf(t, db, invited)...)
	return got
}

// membersOf is the members of userID's household.
func membersOf(t *testing.T, db *store.DB, userID uuid.UUID) []uuid.UUID {
	t.Helper()
	rows, err := db.Read().QueryContext(t.Context(), `SELECT m.user_id FROM household_members AS m
		JOIN household_members AS me ON me.household_id = m.household_id WHERE me.user_id = ? ORDER BY m.user_id`, userID.String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, uuid.MustParse(s))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// An invite goes to another user of the instance outside the household,
// one from a household at a time; each refusal says why.
func TestInviteToHouseholdRefusals(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	eva := addEva(t, db)
	for _, tc := range []struct {
		name            string
		inviter, invite uuid.UUID
		want            error
	}{
		{"no such user", ana, uuid.NewV7(), store.ErrNotFound},
		{"oneself", ana, ana, store.ErrInviteSelf},
	} {
		if _, err := db.InviteToHousehold(t.Context(), tc.inviter, tc.invite, t0); !errors.Is(err, tc.want) {
			t.Errorf("an invite of %s = %v, want %v", tc.name, err, tc.want)
		}
	}
	invite(t, db, ana, ivo)
	if _, err := db.InviteToHousehold(t.Context(), ana, ivo, t0); !errors.Is(err, store.ErrInviteWaiting) {
		t.Errorf("a second invite of Ivo = %v, want ErrInviteWaiting", err)
	}
	// Ivo may invite Ana meanwhile: his own side, alone.
	ivosInvite := invite(t, db, ivo, ana)

	join(t, db, ana, eva)
	if _, err := db.InviteToHousehold(t.Context(), ana, eva, t0); !errors.Is(err, store.ErrInviteMember) {
		t.Errorf("an invite of a member = %v, want ErrInviteMember", err)
	}
	if _, err := db.InviteToHousehold(t.Context(), ivo, eva, t0); !errors.Is(err, store.ErrInviteOtherHousehold) {
		t.Errorf("an invite of another household's member = %v, want ErrInviteOtherHousehold", err)
	}
	// Ana's invite of Ivo is the household's: Eva's waits behind it.
	if _, err := db.InviteToHousehold(t.Context(), eva, ivo, t0); !errors.Is(err, store.ErrInviteWaiting) {
		t.Errorf("Eva's invite of Ivo after Ana's = %v, want ErrInviteWaiting", err)
	}
	// Ivo's invite of Ana waits, but Ana leaves her household first.
	if got, err := db.HouseholdInvitesTo(t.Context(), ana, t0); err != nil || len(got) != 1 || got[0].By.ID != ivo {
		t.Errorf("Ana's invites = %+v, %v; want Ivo's", got, err)
	}
	if _, err := db.PreviewHouseholdInvite(t.Context(), ana, ivosInvite.ID, t0); !errors.Is(err, store.ErrInviteOtherHousehold) {
		t.Errorf("Ana's preview of Ivo's invite = %v, want ErrInviteOtherHousehold", err)
	}
	inviteAfterOneExpired(t, db, eva, ivo)
	if countRows(t, db, "events") != 1 {
		t.Errorf("events = %d, want the join's alone: an invite is none", countRows(t, db, "events"))
	}
}

// inviteAfterOneExpired is eva's invite of ivo once the invite of him
// from her household expired: it waits no more.
func inviteAfterOneExpired(t *testing.T, db *store.DB, eva, ivo uuid.UUID) {
	t.Helper()
	later := t0.Add(store.HouseholdInviteTTL)
	if _, err := db.InviteToHousehold(t.Context(), eva, ivo, later); err != nil {
		t.Errorf("Eva's invite of Ivo after Ana's expired: %v", err)
	}
	if got, err := db.HouseholdInvitesTo(t.Context(), ivo, later); err != nil || len(got) != 1 || got[0].By.ID != eva {
		t.Errorf("Ivo's invites = %+v, %v; want Eva's alone", got, err)
	}
}

// An invite expires after HouseholdInviteTTL: it is listed no more, and
// cannot be accepted.
func TestHouseholdInviteExpires(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	inv := invite(t, db, ana, ivo)
	if !inv.ExpiresAt.Equal(t0.Add(7*24*time.Hour)) || inv.By.ID != ana || inv.Invited.ID != ivo || inv.By.Email != "ana@example.com" {
		t.Errorf("invite = %+v, want Ana's of Ivo for 7 days", inv)
	}
	before := t0.Add(store.HouseholdInviteTTL - time.Second)
	if got, err := db.HouseholdInvitesTo(t.Context(), ivo, before); err != nil || len(got) != 1 {
		t.Errorf("Ivo's invites before it expires = %+v, %v; want Ana's", got, err)
	}
	at := t0.Add(store.HouseholdInviteTTL)
	for name, list := range map[string]func() ([]store.HouseholdInvite, error){
		"Ivo's":       func() ([]store.HouseholdInvite, error) { return db.HouseholdInvitesTo(t.Context(), ivo, at) },
		"Ana's made":  func() ([]store.HouseholdInvite, error) { return db.HouseholdInvitesFrom(t.Context(), ana, at) },
		"Ivo's made:": func() ([]store.HouseholdInvite, error) { return db.HouseholdInvitesFrom(t.Context(), ivo, at) },
	} {
		if got, err := list(); err != nil || len(got) != 0 {
			t.Errorf("%s invites once it expired = %+v, %v; want none", name, got, err)
		}
	}
	if _, err := db.PreviewHouseholdInvite(t.Context(), ivo, inv.ID, at); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("the preview of an expired invite = %v, want ErrNotFound", err)
	}
	if _, _, err := db.AcceptHouseholdInvite(t.Context(), ivo, testActor, inv.ID, "", at); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("the accept of an expired invite = %v, want ErrNotFound", err)
	}
	if got := membersOf(t, db, ivo); len(got) != 0 {
		t.Errorf("Ivo's household = %v, want none", got)
	}
}

// Any member of the inviting household revokes its invite; the invited
// user alone declines it, and accepts it. Each list shows its side.
func TestRevokeAndDeclineHouseholdInvite(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	eva := addEva(t, db)
	join(t, db, ana, ivo)
	inv := invite(t, db, ana, eva)
	if got, err := db.HouseholdInvitesFrom(t.Context(), ivo, t0); err != nil || len(got) != 1 || got[0].ID != inv.ID {
		t.Errorf("the household's invites, as Ivo lists them = %+v, %v; want Ana's of Eva", got, err)
	}
	if got, err := db.HouseholdInvitesFrom(t.Context(), eva, t0); err != nil || len(got) != 0 {
		t.Errorf("Eva's invites made = %+v, %v; want none", got, err)
	}
	if err := db.RevokeHouseholdInvite(t.Context(), eva, inv.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Eva's revoke = %v, want ErrNotFound: she declines", err)
	}
	if err := db.DeclineHouseholdInvite(t.Context(), ivo, inv.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Ivo's decline of Eva's invite = %v, want ErrNotFound", err)
	}
	if _, _, err := db.AcceptHouseholdInvite(t.Context(), ivo, testActor, inv.ID, "", t0); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Ivo's accept of Eva's invite = %v, want ErrNotFound", err)
	}
	if err := db.RevokeHouseholdInvite(t.Context(), ivo, inv.ID); err != nil {
		t.Errorf("Ivo's revoke of Ana's invite: %v", err)
	}
	if err := db.RevokeHouseholdInvite(t.Context(), ivo, inv.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("a second revoke = %v, want ErrNotFound", err)
	}

	inv = invite(t, db, ivo, eva)
	if err := db.DeclineHouseholdInvite(t.Context(), eva, inv.ID); err != nil {
		t.Errorf("Eva's decline: %v", err)
	}
	if got, err := db.HouseholdInvitesTo(t.Context(), eva, t0); err != nil || len(got) != 0 {
		t.Errorf("Eva's invites after her decline = %+v, %v; want none", got, err)
	}
	if countRows(t, db, "events") != 1 {
		t.Errorf("events = %d, want the join's alone: an invite, a revoke and a decline are none", countRows(t, db, "events"))
	}
}

// A join is recorded, and undo refuses it, again and again: the changes
// before it stay.
func TestUndoRefusesAJoin(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	saveFood(t, db, ivo, kefir())
	join(t, db, ana, ivo)
	for i := range 2 {
		if _, err := db.UndoLast(t.Context(), ivo, testActor, t0.Add(time.Minute)); !isConflict(err, store.UndoHousehold) {
			t.Fatalf("Ivo's undo #%d after the join = %v, want UndoHousehold", i+1, err)
		}
	}
	// The same undo sent again is refused again, though a change came
	// since, which it leaves.
	request := uuid.NewV7()
	for i := range 2 {
		if _, _, err := db.Undo(t.Context(), ivo, testActor, request, time.Now()); !isConflict(err, store.UndoHousehold) {
			t.Fatalf("Ivo's undo #%d under a request after the join = %v, want UndoHousehold", i+1, err)
		}
		if i == 0 {
			saveFood(t, db, ivo, chickenBreast())
		}
	}
	copyNamed(t, db, ivo, "Chicken breast")
	copyNamed(t, db, ivo, "Kefir")
	copyNamed(t, db, ana, "Kefir")
}

// HouseholdMembers lists the user's household, them among it, to each of
// its members, and nothing to a user outside it.
func TestHouseholdMembers(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	eva := addEva(t, db)
	if got, err := db.HouseholdMembers(t.Context(), ana); err != nil || len(got) != 0 {
		t.Fatalf("Ana's members before a household = %+v, %v; want none", got, err)
	}
	join(t, db, ana, eva)
	for _, u := range []uuid.UUID{ana, eva} {
		got, err := db.HouseholdMembers(t.Context(), u)
		if err != nil || len(got) != 2 || got[0].ID != ana || got[1].ID != eva || got[1].Email != "eva@example.com" {
			t.Errorf("the members to %v = %+v, %v; want Ana and Eva", u, got, err)
		}
	}
	if got, err := db.HouseholdMembers(t.Context(), ivo); err != nil || len(got) != 0 {
		t.Errorf("Ivo's members = %+v, %v; want none", got, err)
	}
}

// An accept returns the members the user joined, and a leave those who
// stay, each read in its own transaction: whom each tells.
func TestJoinAndLeaveSayWhomToTell(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	eva := addEva(t, db)
	join(t, db, ana, ivo)
	inv := invite(t, db, ivo, eva)
	preview, err := db.PreviewHouseholdInvite(t.Context(), eva, inv.ID, t0)
	if err != nil {
		t.Fatal(err)
	}
	_, mates, err := db.AcceptHouseholdInvite(t.Context(), eva, testActor, inv.ID, preview.Fingerprint, t0)
	if err != nil || len(mates) != 2 || mates[0].ID != ana || mates[1].ID != ivo {
		t.Fatalf("Eva's accept tells %+v, %v; want Ana and Ivo", mates, err)
	}
	stay, err := db.LeaveHousehold(t.Context(), ivo, testActor, t0)
	if err != nil || len(stay) != 2 || stay[0].ID != ana || stay[1].ID != eva {
		t.Fatalf("Ivo's leave tells %+v, %v; want Ana and Eva", stay, err)
	}
	leave(t, db, ana)
	if stay, err := db.LeaveHousehold(t.Context(), eva, testActor, t0); !errors.Is(err, store.ErrNotFound) || len(stay) != 0 {
		t.Errorf("Eva's leave of a household dissolved = %+v, %v; want ErrNotFound", stay, err)
	}
}
