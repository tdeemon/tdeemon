package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/event"
	"github.com/zarnautovic/tdeemon/internal/domain/user"
)

// A user invites another into their household (story 408): the invited
// user accepts, which merges the two kitchens (household_merge.go), or
// declines; the inviting side may revoke the invite. The inviting side
// is the inviter's household, or the inviter alone, with whom the accept
// makes one. Inviting, revoking and declining change no data of a
// user's, and are no events; joining is (household.join).

// HouseholdInviteTTL is how long an invite into a household waits for
// an answer.
const HouseholdInviteTTL = 7 * 24 * time.Hour

// Why an invite into a household cannot be made or accepted.
var (
	// ErrInviteSelf: a user invites another user, not themselves.
	ErrInviteSelf = errors.New("a user invites another user into their household")
	// ErrInviteMember: the invited user is in the inviter's household
	// already.
	ErrInviteMember = errors.New("the user is in this household already")
	// ErrInviteOtherHousehold: the invited user is in another household,
	// which they leave first.
	ErrInviteOtherHousehold = errors.New("the user is in another household")
	// ErrInviteWaiting: an invite of the user from the inviter's household
	// (or the inviter alone) waits already.
	ErrInviteWaiting = errors.New("an invite of this user from this household is waiting")
	// ErrMergeChanged: the merge an accept would do is no longer the one
	// its preview showed; a new preview shows the merge now.
	ErrMergeChanged = errors.New("the household's foods changed since the preview")
)

// HouseholdMembers returns the members of userID's household, userID
// among them, in the order they joined; none when they are in none.
func (db *DB) HouseholdMembers(ctx context.Context, userID uuid.UUID) ([]user.User, error) {
	return db.users(ctx, householdMembers, userID.String(), "")
}

// householdMembers selects the members of user ?1's household but user ?2
// (none for ""), in the order they joined.
const householdMembers = "SELECT " + userColumnsOfU + ` FROM household_members AS m
	JOIN household_members AS me ON me.household_id = m.household_id
	JOIN users AS u ON u.id = m.user_id
	WHERE me.user_id = ?1 AND m.user_id <> ?2 ORDER BY m.joined_at, m.user_id`

// householdMatesInTx returns the members of userID's household but them,
// in tx: those a join or a leave of theirs tells.
func householdMatesInTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID) ([]user.User, error) {
	return queryUsers(ctx, tx, householdMembers, userID.String(), userID.String())
}

// HouseholdPerson is a user an invite names: who invited, or whom.
type HouseholdPerson struct {
	ID    uuid.UUID
	Name  string
	Email string
}

// HouseholdInvite is an invite into a household waiting for an answer:
// By invited Invited's address, until ExpiresAt. Invited's ID is
// uuid.Nil and their Name "" while no user has the address.
type HouseholdInvite struct {
	ID        uuid.UUID
	By        HouseholdPerson
	Invited   HouseholdPerson
	CreatedAt time.Time
	ExpiresAt time.Time
}

// InviteToHousehold invites the address email (normalized,
// user.NormalizeEmail) into userID's household, or into one of the two
// of them when userID is in none, for HouseholdInviteTTL. The invite
// names the address, not a user: whoever has it answers, now or once
// they have an account, and an address of no user is invited as any
// other, so an invite tells nothing about who has an account. userID's
// own address is ErrInviteSelf; one of a member of userID's household
// ErrInviteMember; and one an invite from userID's household (or userID
// alone) waits for already ErrInviteWaiting. An invite expired is no
// longer waiting: a new one takes its place. Whether its user is in
// another household is the accept's to refuse.
func (db *DB) InviteToHousehold(ctx context.Context, userID uuid.UUID, email string, now time.Time) (HouseholdInvite, error) {
	inv := HouseholdInvite{ID: uuid.NewV7(), CreatedAt: now, ExpiresAt: now.Add(HouseholdInviteTTL)}
	err := db.WriteTx(ctx, func(tx *sql.Tx) error {
		var self, member, waiting bool
		if err := tx.QueryRowContext(ctx, `SELECT
				EXISTS (SELECT 1 FROM users WHERE id = ?1 AND email = ?2),
				EXISTS (SELECT 1 FROM users AS u JOIN household_members AS m ON m.user_id = u.id
					JOIN household_members AS me ON me.household_id = m.household_id
					WHERE me.user_id = ?1 AND u.email = ?2),
				EXISTS (SELECT 1 FROM household_invites
					WHERE email = ?2 AND expires_at > ?3 AND user_id IN (`+householdSide+`))`,
			userID.String(), email, formatTime(now)).Scan(&self, &member, &waiting); err != nil {
			return err
		}
		switch {
		case self:
			return ErrInviteSelf
		case member:
			return ErrInviteMember
		case waiting:
			return ErrInviteWaiting
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM household_invites WHERE expires_at <= ?", formatTime(now)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO household_invites (id, user_id, email, created_at, expires_at)
			VALUES (?, ?, ?, ?, ?)`, inv.ID.String(), userID.String(), email,
			formatTime(inv.CreatedAt), formatTime(inv.ExpiresAt)); err != nil {
			return err
		}
		var err error
		inv, err = inviteInTx(ctx, tx, inv.ID, now)
		return err
	})
	if isInviteRefusal(err) {
		return HouseholdInvite{}, err
	}
	if err != nil {
		return HouseholdInvite{}, fmt.Errorf("invite to household: %w", err)
	}
	return inv, nil
}

// householdSide selects the users of the inviting side of user ?1: the
// members of their household, or them alone.
const householdSide = `SELECT ?1 UNION SELECT m.user_id FROM household_members AS m
	JOIN household_members AS me ON me.household_id = m.household_id WHERE me.user_id = ?1`

// checkInvitee refuses invitedID as one userID invites into their
// household: no user (ErrNotFound), a member of it already
// (ErrInviteMember), or of another (ErrInviteOtherHousehold).
func checkInvitee(ctx context.Context, tx *sql.Tx, userID, invitedID uuid.UUID) error {
	var one int
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM users WHERE id = ?", invitedID.String()).Scan(&one); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	theirs, err := householdOf(ctx, tx, invitedID)
	if err != nil || theirs == "" {
		return err
	}
	ours, err := householdOf(ctx, tx, userID)
	if err != nil {
		return err
	}
	if theirs == ours {
		return ErrInviteMember
	}
	return ErrInviteOtherHousehold
}

// isInviteRefusal reports whether err is an invite's refusal, which a
// caller tells apart, rather than a failure: of a merge too, which would
// make a recipe of one side's an ingredient of itself (ErrRecipeCycle).
func isInviteRefusal(err error) bool {
	for _, want := range []error{
		ErrNotFound, ErrInviteSelf, ErrInviteMember, ErrInviteOtherHousehold, ErrInviteWaiting, ErrMergeChanged,
		ErrRecipeCycle,
	} {
		if errors.Is(err, want) {
			return true
		}
	}
	return false
}

// householdInviteSelect selects invites, with the user of the address
// invited when there is one: their id and name are ” when there is
// none.
const householdInviteSelect = `SELECT i.id, i.created_at, i.expires_at,
	b.id, b.name, b.email, coalesce(v.id, ''), coalesce(v.name, ''), i.email
	FROM household_invites AS i
	JOIN users AS b ON b.id = i.user_id
	LEFT JOIN users AS v ON v.email = i.email`

func scanHouseholdInvite(scan func(dest ...any) error) (HouseholdInvite, error) {
	var inv HouseholdInvite
	var id, created, expires, by, invited string
	if err := scan(&id, &created, &expires, &by, &inv.By.Name, &inv.By.Email, &invited, &inv.Invited.Name, &inv.Invited.Email); err != nil {
		return HouseholdInvite{}, err
	}
	if invited == "" {
		invited = uuid.Nil().String()
	}
	var err error
	if inv.ID, err = uuid.Parse(id); err != nil {
		return HouseholdInvite{}, fmt.Errorf("household invite id %q: %w", id, err)
	}
	if inv.By.ID, err = uuid.Parse(by); err != nil {
		return HouseholdInvite{}, fmt.Errorf("household invite user_id %q: %w", by, err)
	}
	if inv.Invited.ID, err = uuid.Parse(invited); err != nil {
		return HouseholdInvite{}, fmt.Errorf("household invite invited_id %q: %w", invited, err)
	}
	if inv.CreatedAt, err = parseTime(created); err != nil {
		return HouseholdInvite{}, err
	}
	if inv.ExpiresAt, err = parseTime(expires); err != nil {
		return HouseholdInvite{}, err
	}
	return inv, nil
}

// inviteInTx returns the invite id while it waits (not expired by now),
// or ErrNotFound.
func inviteInTx(ctx context.Context, tx *sql.Tx, id uuid.UUID, now time.Time) (HouseholdInvite, error) {
	inv, err := scanHouseholdInvite(tx.QueryRowContext(ctx, householdInviteSelect+" WHERE i.id = ? AND i.expires_at > ?",
		id.String(), formatTime(now)).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return HouseholdInvite{}, ErrNotFound
	}
	return inv, err
}

// HouseholdInvitesTo returns the invites into a household waiting for
// userID's answer, the newest first.
func (db *DB) HouseholdInvitesTo(ctx context.Context, userID uuid.UUID, now time.Time) ([]HouseholdInvite, error) {
	return db.householdInvites(ctx, "WHERE i.email = (SELECT email FROM users WHERE id = ?1) AND i.expires_at > ?2", userID, now)
}

// HouseholdInvitesFrom returns the invites waiting that userID's
// household made, any of its members (userID's own when they are in
// none), the newest first.
func (db *DB) HouseholdInvitesFrom(ctx context.Context, userID uuid.UUID, now time.Time) ([]HouseholdInvite, error) {
	return db.householdInvites(ctx, "WHERE i.user_id IN ("+householdSide+") AND i.expires_at > ?2", userID, now)
}

func (db *DB) householdInvites(ctx context.Context, where string, userID uuid.UUID, now time.Time) ([]HouseholdInvite, error) {
	rows, err := db.read.QueryContext(ctx, householdInviteSelect+" "+where+" ORDER BY i.created_at DESC, i.id DESC",
		userID.String(), formatTime(now))
	if err != nil {
		return nil, fmt.Errorf("household invites: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []HouseholdInvite{}
	for rows.Next() {
		inv, err := scanHouseholdInvite(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("household invites: %w", err)
		}
		out = append(out, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("household invites: %w", err)
	}
	return out, nil
}

// RevokeHouseholdInvite takes back the invite id that userID's household
// made: any member of it revokes an invite of any other, as the
// household is the inviting side (userID's own when they are in none).
// An invite of none of theirs, or none at all, is ErrNotFound.
func (db *DB) RevokeHouseholdInvite(ctx context.Context, userID, id uuid.UUID) error {
	return db.deleteHouseholdInvite(ctx, "DELETE FROM household_invites WHERE id = ?2 AND user_id IN ("+householdSide+")",
		userID, id, "revoke household invite")
}

// DeclineHouseholdInvite declines the invite id of userID's, which goes;
// an invite of none of theirs, or none at all, is ErrNotFound.
func (db *DB) DeclineHouseholdInvite(ctx context.Context, userID, id uuid.UUID) error {
	return db.deleteHouseholdInvite(ctx, "DELETE FROM household_invites WHERE id = ?2 AND email = (SELECT email FROM users WHERE id = ?1)",
		userID, id, "decline household invite")
}

// deleteHouseholdInvite deletes the invite id by query, of userID's (?1)
// and the invite's id (?2), and returns ErrNotFound when it deleted none.
func (db *DB) deleteHouseholdInvite(ctx context.Context, query string, userID, id uuid.UUID, op string) error {
	var n int64
	err := db.WriteTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, query, userID.String(), id.String())
		if err != nil {
			return err
		}
		n, err = res.RowsAffected()
		return err
	})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// MergePreview is what accepting an invite would do (PreviewHouseholdInvite):
// the merge's report, and its fingerprint, which the accept is given to do
// that merge and no other.
type MergePreview struct {
	Report      MergeReport
	Fingerprint string
}

// errDryRun rolls back a preview's merge.
var errDryRun = errors.New("a dry run")

// PreviewHouseholdInvite returns what userID's accept of the invite id
// would do now: the merge itself, done and rolled back. An invite of
// none of theirs, or expired, is ErrNotFound; one they cannot accept
// is refused as the accept would be (ErrInviteMember,
// ErrInviteOtherHousehold).
func (db *DB) PreviewHouseholdInvite(ctx context.Context, userID, id uuid.UUID, now time.Time) (MergePreview, error) {
	var out MergePreview
	err := db.WriteTx(ctx, func(tx *sql.Tx) error {
		var err error
		if out, err = joinInTx(ctx, tx, userID, id, now); err != nil {
			return err
		}
		return errDryRun
	})
	switch {
	case errors.Is(err, errDryRun):
		return out, nil
	case isInviteRefusal(err):
		return MergePreview{}, err
	}
	return MergePreview{}, fmt.Errorf("preview household invite: %w", err)
}

// AcceptHouseholdInvite accepts userID's invite id: the merge its preview
// showed (PreviewHouseholdInvite) of the fingerprint given joins userID
// to the inviter's household, made of the two of them when the inviter is
// in none, in one transaction, which takes the invites among its members
// away. It is recorded as userID's event (household.join), which undo
// never takes back. A merge of another fingerprint now is ErrMergeChanged,
// and does nothing; the other refusals are PreviewHouseholdInvite's. It
// returns what the merge did, and the members userID joined, as the
// transaction left them.
func (db *DB) AcceptHouseholdInvite(ctx context.Context, userID uuid.UUID, a event.Actor, id uuid.UUID, fingerprint string,
	now time.Time,
) (MergeReport, []user.User, error) {
	if err := a.Validate(); err != nil {
		return MergeReport{}, nil, fmt.Errorf("invalid actor: %w", err)
	}
	var out MergePreview
	var mates []user.User
	err := db.WriteTx(ctx, func(tx *sql.Tx) error {
		var err error
		if out, err = joinInTx(ctx, tx, userID, id, now); err != nil {
			return err
		}
		if out.Fingerprint != fingerprint {
			return ErrMergeChanged
		}
		if mates, err = householdMatesInTx(ctx, tx, userID); err != nil {
			return err
		}
		_, err = record(ctx, tx, userID, a, event.HouseholdJoin, householdEntity, nil, joinState{Report: out.Report}, now)
		return err
	})
	if isInviteRefusal(err) {
		return MergeReport{}, nil, err
	}
	if err != nil {
		return MergeReport{}, nil, fmt.Errorf("accept household invite: %w", err)
	}
	return out.Report, mates, nil
}

// householdEntity is the entity of a join's event, and a leave's.
const householdEntity = "household"

// joinState is a join as its event records it: what the merge did. It
// names no member, nor the household: the events are the user's, and
// their export.
type joinState struct {
	Report MergeReport `json:"report"`
}

// joinInTx joins userID to the household of the invite id's inviter, in
// tx: the merge of the kitchens (mergeKitchens), the household made when
// the inviter is in none, and the invites among its members gone. It
// returns the merge's report and fingerprint.
func joinInTx(ctx context.Context, tx *sql.Tx, userID, id uuid.UUID, now time.Time) (MergePreview, error) {
	inv, err := inviteInTx(ctx, tx, id, now)
	if err != nil {
		return MergePreview{}, err
	}
	if inv.Invited.ID != userID {
		return MergePreview{}, ErrNotFound
	}
	if err := checkInvitee(ctx, tx, inv.By.ID, userID); err != nil {
		return MergePreview{}, err
	}
	household, err := householdOf(ctx, tx, inv.By.ID)
	if err != nil {
		return MergePreview{}, err
	}
	side := []uuid.UUID{inv.By.ID}
	if household == "" {
		household = uuid.NewV7().String()
		if _, err := tx.ExecContext(ctx, "INSERT INTO households (id, created_at) VALUES (?, ?)", household, formatTime(now)); err != nil {
			return MergePreview{}, err
		}
		if err := addMember(ctx, tx, household, inv.By.ID, now); err != nil {
			return MergePreview{}, err
		}
	} else {
		mates, err := householdMates(ctx, tx, inv.By.ID)
		if err != nil {
			return MergePreview{}, err
		}
		for m := range mates {
			side = append(side, m)
		}
	}
	// The kitchens merge before userID is a member: a write of the merge
	// is none of a household's own.
	out, err := mergeKitchens(ctx, tx, side, userID, now)
	if err != nil {
		return MergePreview{}, err
	}
	if err := addMember(ctx, tx, household, userID, now); err != nil {
		return MergePreview{}, err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM household_invites
		WHERE user_id IN (SELECT user_id FROM household_members WHERE household_id = ?1)
			AND email IN (SELECT u.email FROM users AS u JOIN household_members AS m ON m.user_id = u.id
				WHERE m.household_id = ?1)`, household)
	return out, err
}

// addMember adds userID to household.
func addMember(ctx context.Context, tx *sql.Tx, household string, userID uuid.UUID, now time.Time) error {
	_, err := tx.ExecContext(ctx, "INSERT INTO household_members (user_id, household_id, joined_at) VALUES (?, ?, ?)",
		userID.String(), household, formatTime(now))
	return err
}
