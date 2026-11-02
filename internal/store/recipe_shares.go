package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/event"
	"github.com/zarnautovic/tdeemon/internal/domain/food"
	"github.com/zarnautovic/tdeemon/internal/domain/meal"
	"github.com/zarnautovic/tdeemon/internal/domain/recipe"
)

// A shared recipe (story 403) is one recipe each of its members has:
// every member keeps a recipe of their own (their food, recipes row and
// ingredients), and recipe_shares ties those recipes into a group. A
// save of one writes the shared fields into the others' (syncShared); a
// member who left keeps their recipe for their meals, but picks it no
// more (notWithdrawn).

// notWithdrawn holds a food f to be one its user may pick: none a user
// left the share of. A meal of it reads all the same.
const notWithdrawn = `NOT EXISTS (SELECT 1 FROM recipe_shares AS rs
	WHERE rs.user_id = f.user_id AND rs.food_id = f.id AND rs.left_at IS NOT NULL)`

// RecipeMember is a member of a shared recipe: their seat in its share,
// by which its members name one another, and who they are.
type RecipeMember struct {
	Seat  int
	ID    uuid.UUID
	Name  string
	Email string
}

// withdrawn reports whether userID's food id is a recipe they left the
// share of.
func withdrawn(ctx context.Context, tx *sql.Tx, userID, id uuid.UUID) (bool, error) {
	var left bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM recipe_shares
		WHERE user_id = ? AND food_id = ? AND left_at IS NOT NULL)`, userID.String(), id.String()).Scan(&left)
	return left, err
}

// shareOf returns the group of userID's recipe foodID, with the
// group's version, when userID is a live member of one; "" when the
// recipe is in none, and ErrNotFound when they left it.
func shareOf(ctx context.Context, tx *sql.Tx, userID, foodID uuid.UUID) (string, int64, error) {
	var share string
	var version int64
	var left sql.NullString
	err := tx.QueryRowContext(ctx, "SELECT share_id, version, left_at FROM recipe_shares WHERE user_id = ? AND food_id = ?",
		userID.String(), foodID.String()).Scan(&share, &version, &left)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", 0, nil
	case err != nil:
		return "", 0, err
	case left.Valid:
		return "", 0, ErrNotFound
	}
	return share, version, nil
}

// RecipeMembers returns the live members of the share of userID's recipe
// foodID, userID first and the others by name; nil for a recipe shared
// with no one, or with no one left. A recipe of none of userID's, or one they left, is
// ErrNotFound.
func (db *DB) RecipeMembers(ctx context.Context, userID, foodID uuid.UUID) ([]RecipeMember, error) {
	var out []RecipeMember
	err := db.readTx(ctx, func(tx *sql.Tx) error {
		if _, err := recipeInTx(ctx, tx, userID, foodID); err != nil {
			return err
		}
		share, _, err := shareOf(ctx, tx, userID, foodID)
		if err != nil || share == "" {
			return err
		}
		if out, err = shareMembers(ctx, tx, share, userID); len(out) < 2 {
			out = nil
		}
		return err
	})
	if errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("recipe members: %w", err)
	}
	return out, nil
}

// shareMembers returns the live members of share, first among them
// userID, and the others by name.
func shareMembers(ctx context.Context, tx *sql.Tx, share string, first uuid.UUID) ([]RecipeMember, error) {
	rows, err := tx.QueryContext(ctx, `SELECT rs.seat, u.id, u.name, u.email FROM recipe_shares AS rs JOIN users AS u ON u.id = rs.user_id
		WHERE rs.share_id = ? AND rs.left_at IS NULL ORDER BY u.id <> ?, u.name, u.id`, share, first.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []RecipeMember
	for rows.Next() {
		var m RecipeMember
		var id string
		if err := rows.Scan(&m.Seat, &id, &m.Name, &m.Email); err != nil {
			return nil, err
		}
		if m.ID, err = uuid.Parse(id); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Why a recipe cannot be shared, or a shared one saved, as asked.
var (
	// ErrShareSelf: a recipe is shared with another user, not oneself.
	ErrShareSelf = errors.New("a recipe is shared with another user")
	// ErrShareMember: the user has the recipe already.
	ErrShareMember = errors.New("the user has the recipe already")
	// ErrShareNameTaken: a food of the recipe's name is another member's,
	// or the user's it would be shared with; which food is theirs alone
	// to know.
	ErrShareNameTaken = errors.New("a food of the recipe's name is another member's")
)

// shareState is a member's place in a share, as the events of a share
// and of its stop record it: their recipe, whether they left, and the
// group's version then (the highest of its rows'), which a change of the
// recipe, a share or a stop moves on. A share is never taken away: one
// left with a single live member is that member's ordinary recipe, and
// keeps who left it, whom a share again gives their recipe back.
//
// It names the member by their seat alone, never their user's or their
// food's id: the events are the user's, and their export.
type shareState struct {
	Share   string `json:"share"`
	Seat    int    `json:"seat"`
	Left    bool   `json:"left,omitempty"`
	Version int64  `json:"version"`
}

// seatOf returns the member of share at seat, their recipe, or
// ErrNotFound.
func seatOf(ctx context.Context, tx *sql.Tx, share string, seat int) (shareMember, bool, error) {
	var u, f string
	var left sql.NullString
	err := tx.QueryRowContext(ctx, "SELECT user_id, food_id, left_at FROM recipe_shares WHERE share_id = ? AND seat = ?",
		share, seat).Scan(&u, &f, &left)
	if errors.Is(err, sql.ErrNoRows) {
		return shareMember{}, false, ErrNotFound
	}
	if err != nil {
		return shareMember{}, false, err
	}
	m := shareMember{}
	if m.User, err = uuid.Parse(u); err == nil {
		m.Food, err = uuid.Parse(f)
	}
	return m, left.Valid, err
}

// insertSeat adds userID's recipe foodID to share, at its next seat,
// which it returns: past every seat and version the share has had, so a
// seat is never one of a member before (whose account went, taking their
// row along).
func insertSeat(ctx context.Context, tx *sql.Tx, share string, userID, foodID uuid.UUID) (int, error) {
	var seat int
	err := tx.QueryRowContext(ctx, `INSERT INTO recipe_shares (share_id, seat, user_id, food_id, version)
		VALUES (?1, (SELECT max(coalesce(max(seat), 0), coalesce(max(version), 0)) + 1 FROM recipe_shares WHERE share_id = ?1), ?2, ?3,
			(SELECT coalesce(max(version), 1) FROM recipe_shares WHERE share_id = ?1))
		RETURNING seat`, share, userID.String(), foodID.String()).Scan(&seat)
	return seat, err
}

// shareMember is a member's recipe in a share.
type shareMember struct {
	User uuid.UUID `json:"user"`
	Food uuid.UUID `json:"food"`
}

// liveMembers returns the live members of share but userID.
func liveMembers(ctx context.Context, tx *sql.Tx, share string, but uuid.UUID) ([]shareMember, error) {
	rows, err := tx.QueryContext(ctx, "SELECT user_id, food_id FROM recipe_shares WHERE share_id = ? AND left_at IS NULL AND user_id <> ?",
		share, but.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []shareMember
	for rows.Next() {
		var u, f string
		if err := rows.Scan(&u, &f); err != nil {
			return nil, err
		}
		m := shareMember{}
		if m.User, err = uuid.Parse(u); err != nil {
			return nil, err
		}
		if m.Food, err = uuid.Parse(f); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// bumpShare moves share's version on, in its live rows, and returns it.
func bumpShare(ctx context.Context, tx *sql.Tx, share string) (int64, error) {
	if _, err := tx.ExecContext(ctx, "UPDATE recipe_shares SET version = (SELECT max(version) FROM recipe_shares WHERE share_id = ?1) + 1 WHERE share_id = ?1 AND left_at IS NULL",
		share); err != nil {
		return 0, err
	}
	return shareVersion(ctx, tx, share)
}

// shareVersion is share's version: the highest of its rows'.
func shareVersion(ctx context.Context, tx *sql.Tx, share string) (int64, error) {
	var v sql.NullInt64
	err := tx.QueryRowContext(ctx, "SELECT max(version) FROM recipe_shares WHERE share_id = ?", share).Scan(&v)
	return v.Int64, err
}

// memberRecipe is r, a recipe as member from saved it, as member to has
// it: of to's food id, with to's note, aliases, barcodes and archived,
// and each ingredient's food named by its owner as to's recipe names it.
func memberRecipe(r recipe.Recipe, from uuid.UUID, to shareMember, own food.Food) recipe.Recipe {
	out := r
	out.Food.ID = to.Food
	out.Food.Note, out.Food.Aliases, out.Food.Barcodes, out.Food.Archived = own.Note, own.Aliases, own.Barcodes, own.Archived
	out.Items = make([]meal.Item, len(r.Items))
	for i := range r.Items {
		it := r.Items[i]
		owner := it.FoodUser
		if owner == uuid.Nil() {
			owner = from
		}
		if owner == to.User {
			owner = uuid.Nil()
		}
		it.FoodUser = owner
		out.Items[i] = it
	}
	return out
}

// writeMemberRecipe writes r, as member from saved it, into member to's
// recipe: the shared fields, to's own kept. A food of the name that is
// another of to's is ErrShareNameTaken.
func writeMemberRecipe(ctx context.Context, tx *sql.Tx, r recipe.Recipe, from uuid.UUID, to shareMember, own food.Food, now time.Time) error {
	mr := memberRecipe(r, from, to, own)
	nameKey := food.Key(mr.Food.Name)
	if err := checkNameFree(ctx, tx, to.User, to.Food, nameKey); err != nil {
		if errors.Is(err, ErrNameTaken) {
			return ErrShareNameTaken
		}
		return err
	}
	if err := updateFood(ctx, tx, to.User, mr.Food, nameKey, "", now); err != nil {
		return err
	}
	return writeRecipe(ctx, tx, to.User, mr)
}

// syncShared writes userID's recipe r, of their food foodID, into every
// other live member's of its share, and moves the share's version on;
// a recipe in no share is left as it is.
func syncShared(ctx context.Context, tx *sql.Tx, userID, foodID uuid.UUID, r recipe.Recipe, now time.Time) error {
	share, _, err := shareOf(ctx, tx, userID, foodID)
	if err != nil || share == "" {
		return err
	}
	others, err := liveMembers(ctx, tx, share, userID)
	if err != nil {
		return err
	}
	for _, m := range others {
		own, err := foodInTx(ctx, tx, m.User, m.Food)
		if err != nil {
			return err
		}
		if err := writeMemberRecipe(ctx, tx, r, userID, m, own, now); err != nil {
			return err
		}
	}
	_, err = bumpShare(ctx, tx, share)
	return err
}

// ShareRecipe shares userID's recipe foodID with the user withID, who
// gets a recipe of their own of it at once: its shared fields (the name,
// the ingredients, the dish's weight, the portions, the pot), no note,
// aliases or barcodes of anyone's. userID is a live member of its share,
// or has it alone (the share is made then). A recipe of none of userID's,
// or one they left, is ErrNotFound; withID of no user is ErrNotFound
// too. A user who left the share before gets their recipe back. It is
// recorded as userID's event (recipe.share).
func (db *DB) ShareRecipe(ctx context.Context, userID uuid.UUID, a event.Actor, foodID, withID uuid.UUID, now time.Time) error {
	if err := a.Validate(); err != nil {
		return fmt.Errorf("invalid actor: %w", err)
	}
	if withID == userID {
		return ErrShareSelf
	}
	err := db.WriteTx(ctx, func(tx *sql.Tx) error {
		return shareRecipeInTx(ctx, tx, userID, a, foodID, withID, now)
	})
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrShareMember), errors.Is(err, ErrShareNameTaken):
		return err
	case err != nil:
		return fmt.Errorf("share recipe: %w", err)
	}
	return nil
}

func shareRecipeInTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID, a event.Actor, foodID, withID uuid.UUID, now time.Time) error {
	r, share, err := shareFrom(ctx, tx, userID, foodID, withID)
	if err != nil {
		return err
	}
	to, seat, back, err := recipient(ctx, tx, share, withID)
	if err != nil {
		return err
	}
	if err := checkNameFree(ctx, tx, withID, to.Food, food.Key(r.Food.Name)); errors.Is(err, ErrNameTaken) {
		return ErrShareNameTaken
	} else if err != nil {
		return err
	}
	if err := giveRecipe(ctx, tx, r, userID, to, back, now); err != nil {
		return err
	}
	if back {
		_, err = tx.ExecContext(ctx, "UPDATE recipe_shares SET left_at = NULL WHERE share_id = ? AND seat = ?", share, seat)
	} else {
		seat, err = insertSeat(ctx, tx, share, withID, to.Food)
	}
	if err != nil {
		return err
	}
	version, err := bumpShare(ctx, tx, share)
	if err != nil {
		return err
	}
	_, err = record(ctx, tx, userID, a, event.RecipeShare, foodID.String(), nil,
		shareState{Share: share, Seat: seat, Version: version}, now)
	return err
}

// shareFrom returns userID's recipe foodID, which they share with withID,
// and its share, made when it has none. A recipe of none of userID's, one
// they left, or withID of no user is ErrNotFound.
func shareFrom(ctx context.Context, tx *sql.Tx, userID, foodID, withID uuid.UUID) (recipe.Recipe, string, error) {
	if left, err := withdrawn(ctx, tx, userID, foodID); err != nil || left {
		return recipe.Recipe{}, "", cmpErr(err, left)
	}
	r, err := recipeInTx(ctx, tx, userID, foodID)
	if err != nil {
		return recipe.Recipe{}, "", err
	}
	var one int
	if err := tx.QueryRowContext(ctx, "SELECT 1 FROM users WHERE id = ?", withID.String()).Scan(&one); errors.Is(err, sql.ErrNoRows) {
		return recipe.Recipe{}, "", ErrNotFound
	} else if err != nil {
		return recipe.Recipe{}, "", err
	}
	share, _, err := shareOf(ctx, tx, userID, foodID)
	if err != nil || share != "" {
		return r, share, err
	}
	share = uuid.NewV7().String()
	_, err = insertSeat(ctx, tx, share, userID, foodID)
	return r, share, err
}

// giveRecipe writes userID's recipe r into to's: the one they had of it
// (back), or a new food of theirs.
func giveRecipe(ctx context.Context, tx *sql.Tx, r recipe.Recipe, userID uuid.UUID, to shareMember, back bool, now time.Time) error {
	own := food.Food{ID: to.Food}
	if back {
		var err error
		if own, err = foodInTx(ctx, tx, to.User, to.Food); err != nil {
			return err
		}
	} else if err := insertFood(ctx, tx, to.User, food.Food{ID: to.Food, Name: r.Food.Name, Basis: r.Food.Basis, Source: food.Recipe},
		food.Key(r.Food.Name), "", now); err != nil {
		return err
	}
	return writeMemberRecipe(ctx, tx, r, userID, to, own, now)
}

// cmpErr is err, or ErrNotFound when there is none but the recipe is
// one its user left.
func cmpErr(err error, left bool) error {
	if err == nil && left {
		return ErrNotFound
	}
	return err
}

// recipient is the recipe withID gets of share: the one they had of it
// and left (back), at their seat, or a new one. A live member is
// ErrShareMember.
func recipient(ctx context.Context, tx *sql.Tx, share string, withID uuid.UUID) (shareMember, int, bool, error) {
	var f string
	var seat int
	var left sql.NullString
	err := tx.QueryRowContext(ctx, "SELECT food_id, seat, left_at FROM recipe_shares WHERE share_id = ? AND user_id = ?",
		share, withID.String()).Scan(&f, &seat, &left)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return shareMember{User: withID, Food: uuid.NewV7()}, 0, false, nil
	case err != nil:
		return shareMember{}, 0, false, err
	case !left.Valid:
		return shareMember{}, 0, false, ErrShareMember
	}
	id, err := uuid.Parse(f)
	return shareMember{User: withID, Food: id}, seat, true, err
}

// UnshareRecipe takes userID's recipe foodID from the live member of its
// share at seat, userID themselves too (leaving it): their recipe
// stays for their meals, but they pick it no more. A share left with one
// live member is that member's ordinary recipe. A recipe in no share of
// userID's, or a member who is none, is ErrNotFound. It is recorded as userID's event (recipe.unshare).
func (db *DB) UnshareRecipe(ctx context.Context, userID uuid.UUID, a event.Actor, foodID uuid.UUID, seat int, now time.Time) error {
	if err := a.Validate(); err != nil {
		return fmt.Errorf("invalid actor: %w", err)
	}
	err := db.WriteTx(ctx, func(tx *sql.Tx) error {
		share, version, err := shareOf(ctx, tx, userID, foodID)
		if err != nil {
			return err
		}
		if share == "" {
			return ErrNotFound
		}
		if _, left, err := seatOf(ctx, tx, share, seat); err != nil || left {
			return cmpErr(err, left)
		}
		before := shareState{Share: share, Seat: seat, Version: version}
		after, err := leave(ctx, tx, share, seat, now)
		if err != nil {
			return err
		}
		_, err = record(ctx, tx, userID, a, event.RecipeUnshare, foodID.String(), before, after, now)
		return err
	})
	if errors.Is(err, ErrNotFound) {
		return err
	}
	if err != nil {
		return fmt.Errorf("unshare recipe: %w", err)
	}
	return nil
}

// leave takes the member at seat out of share: the group's version moves
// on, and they left. It returns the member's state after.
func leave(ctx context.Context, tx *sql.Tx, share string, seat int, now time.Time) (shareState, error) {
	version, err := bumpShare(ctx, tx, share)
	if err != nil {
		return shareState{}, err
	}
	_, err = tx.ExecContext(ctx, "UPDATE recipe_shares SET left_at = ? WHERE share_id = ? AND seat = ?",
		formatTime(now), share, seat)
	return shareState{Share: share, Seat: seat, Left: true, Version: version}, err
}

// shareStateNow returns the state of the member a share or its stop
// was of, as the event records it: nil when their recipe is gone.
func shareStateNow(ctx context.Context, tx *sql.Tx, e event.Event) (any, error) {
	var s shareState
	if err := decode(e.After, &s); err != nil {
		return nil, fmt.Errorf("%w: %w", errUnreadable, err)
	}
	_, left, err := seatOf(ctx, tx, s.Share, s.Seat)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	now := shareState{Share: s.Share, Seat: s.Seat, Left: left}
	if now.Version, err = shareVersion(ctx, tx, s.Share); err != nil {
		return nil, err
	}
	return now, nil
}

// takeBackShare takes the recipe of the share e from the member it gave
// it to, that member alone: their recipe goes when nothing of theirs
// uses it, else it stays, left, for their meals.
func takeBackShare(ctx context.Context, tx *sql.Tx, e event.Event, now time.Time) error {
	var s shareState
	if err := decode(e.After, &s); err != nil {
		return fmt.Errorf("%w: %w", errUnreadable, err)
	}
	m, _, err := seatOf(ctx, tx, s.Share, s.Seat)
	if err != nil {
		return err
	}
	used, err := foodInUse(ctx, tx, m.User, m.Food)
	if err != nil {
		return err
	}
	if used {
		if _, err := leave(ctx, tx, s.Share, s.Seat, now); err != nil {
			return err
		}
	} else if _, err := tx.ExecContext(ctx, "DELETE FROM foods WHERE user_id = ? AND id = ?", m.User.String(), m.Food.String()); err != nil {
		return err
	}
	// The version before the share, which moved it on by one.
	return setShareVersion(ctx, tx, s.Share, s.Version-1)
}

// takeBackUnshare gives the member the stop e took the recipe from their
// recipe back, of the share's shared fields now.
func takeBackUnshare(ctx context.Context, tx *sql.Tx, userID uuid.UUID, e event.Event, now time.Time) error {
	var s shareState
	if err := decode(e.After, &s); err != nil {
		return fmt.Errorf("%w: %w", errUnreadable, err)
	}
	m, _, err := seatOf(ctx, tx, s.Share, s.Seat)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE recipe_shares SET left_at = NULL WHERE share_id = ? AND seat = ?",
		s.Share, s.Seat); err != nil {
		return err
	}
	others, err := liveMembers(ctx, tx, s.Share, m.User)
	if err != nil {
		return err
	}
	if len(others) == 0 {
		return clash{UndoStuck}
	}
	from := others[0]
	for _, m := range others {
		if m.User == userID {
			from = m
		}
	}
	r, err := recipeInTx(ctx, tx, from.User, from.Food)
	if err != nil {
		return err
	}
	own, err := foodInTx(ctx, tx, m.User, m.Food)
	if err != nil {
		return err
	}
	if err := writeMemberRecipe(ctx, tx, r, from.User, m, own, now); errors.Is(err, ErrShareNameTaken) {
		return fmt.Errorf("%w: %w", clash{UndoNameTaken}, err)
	} else if err != nil {
		return err
	}
	var before shareState
	if err := decode(e.Before, &before); err != nil {
		return fmt.Errorf("%w: %w", errUnreadable, err)
	}
	return setShareVersion(ctx, tx, s.Share, before.Version)
}

// adoptIngredients gives every other member whose shared recipe has an
// ingredient of userID's food, userID's account about to go, a food of
// their own of it, which the ingredient then names: its name, brand,
// values and servings, no note, aliases or barcodes of userID's. A food
// of theirs of its name and brand is that food. Nothing of it is a
// change of theirs to undo.
func adoptIngredients(ctx context.Context, tx *sql.Tx, userID uuid.UUID) error {
	todo, err := foreignIngredients(ctx, tx, userID)
	if err != nil {
		return err
	}
	now := time.Now()
	type key struct {
		to, food uuid.UUID
		frozen   bool
	}
	made := map[key]uuid.UUID{}
	for _, a := range todo {
		f, frozen, err := adoptedFrom(ctx, tx, a)
		if err != nil {
			return err
		}
		k := key{a.User, a.Food, frozen}
		id, ok := made[k]
		if !ok {
			if id, err = adoptFood(ctx, tx, a.User, f, now); err != nil {
				return err
			}
			made[k] = id
		}
		if _, err := tx.ExecContext(ctx, `UPDATE recipe_items SET food_user_id = ?, food_id = ?
			WHERE user_id = ? AND recipe_id = ? AND food_user_id = ? AND food_id = ?`,
			a.User.String(), id.String(), a.User.String(), a.Recipe.String(), userID.String(), a.Food.String()); err != nil {
			return err
		}
	}
	return nil
}

// adoptedFrom is the food of the user going that the recipe of a has as
// an ingredient, as a save of it reads it: their food as it is while
// they are a live member of its share, else the values the recipe keeps
// (frozenFood), which a later change of theirs never reaches.
func adoptedFrom(ctx context.Context, tx *sql.Tx, a adoption) (food.Food, bool, error) {
	share, _, err := shareOf(ctx, tx, a.User, a.Recipe)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return food.Food{}, false, err
	}
	stored, err := storedItems(ctx, tx, a.User, a.Recipe)
	if err != nil {
		return food.Food{}, false, err
	}
	it := stored[a.Food]
	f, _, frozen, err := ingredientSource(ctx, tx, a.User, share, stored, &it)
	return f, frozen, err
}

// adoption is an ingredient of another user's recipe of a food that is
// to be theirs.
type adoption struct {
	User, Recipe, Food uuid.UUID
}

// foreignIngredients returns the ingredients of other users' recipes of
// userID's foods.
func foreignIngredients(ctx context.Context, tx *sql.Tx, userID uuid.UUID) ([]adoption, error) {
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT user_id, recipe_id, food_id FROM recipe_items
		WHERE food_user_id = ?1 AND user_id <> ?1`, userID.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []adoption
	for rows.Next() {
		var u, r, f string
		if err := rows.Scan(&u, &r, &f); err != nil {
			return nil, err
		}
		a := adoption{}
		if a.User, err = uuid.Parse(u); err == nil {
			if a.Recipe, err = uuid.Parse(r); err == nil {
				a.Food, err = uuid.Parse(f)
			}
		}
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// adoptFood makes a copy of f userID's, and returns its id: never a food
// of theirs of its name (which may be made of the recipe that has f, or
// lack its servings), but the name with a number after it when theirs
// has it. A recipe's food is a plain food of its values then: its recipe
// stays its user's.
func adoptFood(ctx context.Context, tx *sql.Tx, userID uuid.UUID, f food.Food, now time.Time) (uuid.UUID, error) {
	f.ID, f.Note, f.Aliases, f.Barcodes, f.Archived = uuid.NewV7(), "", nil, nil, false
	if f.Source == food.Recipe {
		f.Source = food.User
	}
	name := []rune(f.Name)
	for n := 2; ; n++ {
		var taken bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM foods WHERE user_id = ? AND name_key = ? AND brand_key = ?)",
			userID.String(), food.Key(f.Name), food.Key(f.Brand)).Scan(&taken); err != nil {
			return uuid.Nil(), err
		}
		if !taken {
			break
		}
		suffix := fmt.Sprintf(" (%d)", n)
		base := name[:min(len(name), food.MaxNameLen-len(suffix))]
		f.Name = string(base) + suffix
	}
	if err := insertFood(ctx, tx, userID, f, food.Key(f.Name), food.Key(f.Brand), now); err != nil {
		return uuid.Nil(), err
	}
	return f.ID, replaceFoodChildren(ctx, tx, userID, f)
}

// sharedWithOthers reports whether userID's recipe foodID is in a share
// with another live member.
func sharedWithOthers(ctx context.Context, tx *sql.Tx, userID, foodID uuid.UUID) (bool, error) {
	share, _, err := shareOf(ctx, tx, userID, foodID)
	if err != nil || share == "" {
		return false, err
	}
	others, err := liveMembers(ctx, tx, share, userID)
	return len(others) > 0, err
}

// withoutWithdrawn returns ms but the meals of a food of userID's that is
// a recipe they left.
func (db *DB) withoutWithdrawn(ctx context.Context, userID uuid.UUID, ms []meal.Meal) ([]meal.Meal, error) {
	rows, err := db.read.QueryContext(ctx, "SELECT food_id FROM recipe_shares WHERE user_id = ? AND left_at IS NOT NULL", userID.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	left := map[uuid.UUID]bool{}
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(f)
		if err != nil {
			return nil, err
		}
		left[id] = true
	}
	if err := rows.Err(); err != nil || len(left) == 0 {
		return ms, err
	}
	return slices.DeleteFunc(ms, func(m meal.Meal) bool {
		return slices.ContainsFunc(m.Items, func(it meal.Item) bool { return left[it.FoodID] })
	}), nil
}

// ingredientSource returns the food a save of userID's recipe takes the
// values of its ingredient it from, and its owner when another member's.
// stored is the recipe's ingredients as saved, by their food's id: one of
// another member's food is read as that member has it while they are a
// live member of share and it is not archived; else, and for a food of
// userID's that is a recipe they left, or, in a shared recipe, archived,
// the ingredient is frozen: it keeps the values the recipe has, and the
// food returned is made of them. Any other is userID's food as it is.
func ingredientSource(ctx context.Context, tx *sql.Tx, userID uuid.UUID, share string, stored map[uuid.UUID]meal.Item,
	it *meal.Item,
) (f food.Food, owner uuid.UUID, frozen bool, err error) {
	had, known := stored[it.FoodID]
	if !known || had.FoodUser == uuid.Nil() {
		f, err := foodInTx(ctx, tx, userID, it.FoodID)
		if err != nil || !known {
			return f, uuid.Nil(), false, err
		}
		left, err := withdrawn(ctx, tx, userID, it.FoodID)
		if err != nil {
			return food.Food{}, uuid.Nil(), false, err
		}
		if left || (share != "" && f.Archived) {
			return frozenFood(had), uuid.Nil(), true, nil
		}
		return f, uuid.Nil(), false, nil
	}
	f, err = foodInTx(ctx, tx, had.FoodUser, it.FoodID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return food.Food{}, uuid.Nil(), false, err
	}
	var live bool
	if err == nil && !f.Archived {
		if live, err = liveIn(ctx, tx, share, had.FoodUser); err != nil {
			return food.Food{}, uuid.Nil(), false, err
		}
	}
	if live {
		return f, had.FoodUser, false, nil
	}
	return frozenFood(had), had.FoodUser, true, nil
}

// liveIn reports whether userID is a live member of share.
func liveIn(ctx context.Context, tx *sql.Tx, share string, userID uuid.UUID) (bool, error) {
	var live bool
	err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM recipe_shares WHERE share_id = ? AND user_id = ? AND left_at IS NULL)",
		share, userID.String()).Scan(&live)
	return live, err
}

// frozenFood is the food an ingredient's values make: per 100 g or 100 ml
// of its amount, or per portion, with the serving its quantity names, of
// the amount it came to, so its quantity as given still reads.
func frozenFood(it meal.Item) food.Food {
	f := food.Food{ID: it.FoodID, Name: it.Name, Brand: it.Brand, Source: it.Source, PortionGrams: it.PortionGrams}
	if q := it.Quantity; q.Servings > 0 && it.Unit != "portion" {
		name := q.Serving
		if name == "" {
			name = "serving"
		}
		f.Servings = []food.Serving{{Name: name, Amount: it.Amount / q.Servings}}
	}
	per := 100 / it.Amount
	switch it.Unit {
	case "ml":
		f.Basis = food.Per100ml
	case "portion":
		f.Basis, per = food.PerPortion, 1/it.Amount
	default:
		f.Basis = food.Per100g
	}
	f.Nutrients = it.Nutrients.Scale(per)
	return f
}

// storedItems is the ingredients of userID's recipe id as saved, by
// their food's id: whose food each is, and the values it keeps when
// frozen. None for a new recipe.
func storedItems(ctx context.Context, tx *sql.Tx, userID, id uuid.UUID) (map[uuid.UUID]meal.Item, error) {
	r, err := recipeInTx(ctx, tx, userID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]meal.Item{}
	for i := range r.Items {
		if it := &r.Items[i]; it.FoodID != uuid.Nil() {
			out[it.FoodID] = *it
		}
	}
	return out, nil
}

// IngredientFoods returns, by position, the food each ingredient of
// userID's recipe id takes its values from in a save (ingredientSource):
// none for one that keeps its own, frozen, nor for one of a food gone.
func (db *DB) IngredientFoods(ctx context.Context, userID, id uuid.UUID) (map[int]food.Food, error) {
	out := map[int]food.Food{}
	err := db.readTx(ctx, func(tx *sql.Tx) error {
		r, err := recipeInTx(ctx, tx, userID, id)
		if err != nil {
			return err
		}
		share, _, err := shareOf(ctx, tx, userID, id)
		if err != nil {
			return err
		}
		stored, err := storedItems(ctx, tx, userID, id)
		if err != nil {
			return err
		}
		for i := range r.Items {
			it := &r.Items[i]
			if it.FoodID == uuid.Nil() {
				continue
			}
			f, _, frozen, err := ingredientSource(ctx, tx, userID, share, stored, it)
			if errors.Is(err, ErrNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			if !frozen {
				out[i] = f
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("ingredient foods: %w", err)
	}
	return out, nil
}

// setShareVersion puts share's version back to v, an undo's: every row
// past it comes back to it, so the change before, which left it, is the
// latest again.
func setShareVersion(ctx context.Context, tx *sql.Tx, share string, v int64) error {
	_, err := tx.ExecContext(ctx, "UPDATE recipe_shares SET version = ?2 WHERE share_id = ?1 AND version > ?2", share, v)
	return err
}
