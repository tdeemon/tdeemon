package store

import (
	"context"
	"database/sql"
	_ "embed" // The statements of a leave.
	"errors"
	"fmt"
	"strings"
	"time"
	"uuid"

	"github.com/tdeemon/tdeemon/internal/domain/event"
	"github.com/tdeemon/tdeemon/internal/domain/user"
)

// A member leaves their household (story 408), and no one else takes
// them out of it: the maintainer's call. Every member keeps every food
// and recipe they have, as it is: the one who goes keeps theirs as
// ordinary foods of their own, their household seats in food_shares
// gone; the others keep theirs, still one kitchen when two or more of
// them stay, and a household of one is dissolved. A recipe a member is
// in by an explicit share (story 403) keeps them in its group; the
// others, there by the household's seats alone, take their copies to a
// group of their own. The statements are household_leave.sql, which
// docs/DESIGN.md gives an admin who must take a member out by hand.
//
// Each member's recipes name their own copies of the household's foods
// already (ownIngredients): a leave changes no food, nor a recipe. An
// ingredient of another member's food, which has no copy of theirs (a
// recipe its owner left), is read at the values the recipe keeps from
// then on, as a shared recipe's of a member who left it.

//go:embed household_leave.sql
var householdLeaveSQL string

// householdLeaveStatements are the statements of household_leave.sql,
// one at a time, as the driver runs them.
var householdLeaveStatements = scriptStatements(householdLeaveSQL)

// scriptStatements splits script at each ";" that ends a line, and drops
// what holds no statement but comments: of a script of ours, whose
// literals hold no such ";" (splitStatements reads any, but blanks them).
func scriptStatements(script string) []string {
	var out []string
	for s := range strings.SplitSeq(script, ";\n") {
		code := false
		for line := range strings.SplitSeq(s, "\n") {
			if l := strings.TrimSpace(line); l != "" && !strings.HasPrefix(l, "--") {
				code = true
			}
		}
		if code {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// leaveState is a leave as its event records it: whether the household
// was dissolved, one member left in it. It names no member, nor the
// household: the events are the user's, and their export.
type leaveState struct {
	Dissolved bool `json:"dissolved,omitempty"`
}

// LeaveHousehold takes userID out of their household, as the package's
// comment above says, in one transaction, which takes the invites they
// made away. It is recorded as userID's event (household.leave), which
// undo never takes back, nor goes past. A user in no household is
// ErrNotFound. It returns the members who stay, as the transaction found
// them.
func (db *DB) LeaveHousehold(ctx context.Context, userID uuid.UUID, a event.Actor, now time.Time) ([]user.User, error) {
	if err := a.Validate(); err != nil {
		return nil, fmt.Errorf("invalid actor: %w", err)
	}
	var stay []user.User
	err := db.WriteTx(ctx, func(tx *sql.Tx) error {
		var err error
		if stay, err = householdMatesInTx(ctx, tx, userID); err != nil {
			return err
		}
		dissolved, err := leaveHousehold(ctx, tx, userID)
		if err != nil {
			return err
		}
		_, err = record(ctx, tx, userID, a, event.HouseholdLeave, householdEntity, nil, leaveState{Dissolved: dissolved}, now)
		return err
	})
	if errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("leave household: %w", err)
	}
	return stay, nil
}

// leaveHousehold takes userID out of their household in tx, as
// household_leave.sql does, and reports whether it dissolved it. A user
// in no household is ErrNotFound.
func leaveHousehold(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (bool, error) {
	household, err := householdOf(ctx, tx, userID)
	if err != nil {
		return false, err
	}
	if household == "" {
		return false, ErrNotFound
	}
	mates, err := householdMates(ctx, tx, userID)
	if err != nil {
		return false, err
	}
	for _, s := range householdLeaveStatements {
		var args []any
		if strings.Contains(s, ":user") {
			args = append(args, sql.Named("user", userID.String()))
		}
		if _, err := tx.ExecContext(ctx, s, args...); err != nil {
			return false, fmt.Errorf("%.40q: %w", s, err)
		}
	}
	return len(mates) < 2, nil
}
