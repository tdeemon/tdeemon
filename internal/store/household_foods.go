package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/food"
	"github.com/zarnautovic/tdeemon/internal/domain/meal"
)

// A household (story 408) keeps one kitchen: every food of a member,
// recipes and estimates too, is a food of each other member's, their
// copy, and a share group (food_shares, in household seats) ties a
// food's copies together, as a shared recipe's (story 403). A save of a
// member's food writes its shared fields into the others' copies in the
// same transaction (spreadFood, and syncShared for a recipe), making the
// copies a member lacks: its name, brand, basis, values, servings,
// portion, source and barcodes, and a recipe's ingredients, dish's
// weight, portions and pot, each ingredient of the member's own copy of
// its food. A member's note, aliases and archived stay theirs, and so do
// their meals, and with them how often they eat a food. Only the copies
// of the members change: a user outside the household is never written.

// householdMates returns the other members of userID's household, by
// id; none when they are in none.
func householdMates(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (map[uuid.UUID]bool, error) {
	rows, err := tx.QueryContext(ctx, `SELECT m.user_id FROM household_members AS m
		JOIN household_members AS me ON me.household_id = m.household_id
		WHERE me.user_id = ?1 AND m.user_id <> ?1`, userID.String())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[uuid.UUID]bool{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// householdSeats gives each other member of userID's household a seat
// in the group of userID's food f, which it makes when there is none, in
// a household seat: the seat they have, brought back if they left it, or
// a new food of theirs of f's name, brand, basis and source, archived as
// f is, which the save then writes whole. It returns the group, "" when
// userID is in no household or left the food, and the members it seated
// (fresh), whose food is to be written whole. A food of the name and brand that is
// another of a member's is ErrShareNameTaken.
func householdSeats(ctx context.Context, tx *sql.Tx, userID uuid.UUID, f food.Food, now time.Time) (string, []shareMember, error) {
	mates, err := householdMates(ctx, tx, userID)
	if err != nil || len(mates) == 0 {
		return "", nil, err
	}
	share, _, err := shareOf(ctx, tx, userID, f.ID)
	switch {
	case errors.Is(err, ErrNotFound):
		// A recipe they left is theirs alone, for their meals.
		return "", nil, nil
	case err != nil:
		return "", nil, err
	case share == "":
		share = uuid.NewV7().String()
		if _, err := insertSeat(ctx, tx, share, userID, f.ID, true); err != nil {
			return "", nil, err
		}
	}
	var fresh []shareMember
	for m := range mates {
		to, seated, err := seatMate(ctx, tx, share, m, f, now)
		if err != nil {
			return "", nil, err
		}
		if seated {
			fresh = append(fresh, to)
		}
	}
	return share, fresh, nil
}

// seatMate gives member m a seat in share of food f, as householdSeats
// does: it returns their food of it, and whether it seated them (a new
// seat, or one they left brought back), not a live one they have. A new
// seat's food is archived as f is: a food saved withdrawn comes to no
// one in use.
func seatMate(ctx context.Context, tx *sql.Tx, share string, m uuid.UUID, f food.Food, now time.Time) (shareMember, bool, error) {
	var id string
	var left sql.NullString
	err := tx.QueryRowContext(ctx, "SELECT food_id, left_at FROM food_shares WHERE share_id = ? AND user_id = ?",
		share, m.String()).Scan(&id, &left)
	switch {
	case err == nil && !left.Valid:
		return shareMember{}, false, nil
	case err == nil:
		if _, err := tx.ExecContext(ctx, "UPDATE food_shares SET left_at = NULL, household = 1 WHERE share_id = ? AND user_id = ?",
			share, m.String()); err != nil {
			return shareMember{}, false, err
		}
		back, err := uuid.Parse(id)
		return shareMember{User: m, Food: back}, true, err
	case !errors.Is(err, sql.ErrNoRows):
		return shareMember{}, false, err
	}
	to := shareMember{User: m, Food: uuid.NewV7()}
	if err := checkMemberName(ctx, tx, to, f); err != nil {
		return shareMember{}, false, err
	}
	if err := insertFood(ctx, tx, m, food.Food{ID: to.Food, Name: f.Name, Brand: f.Brand, Basis: f.Basis, Source: f.Source, Archived: f.Archived},
		food.Key(f.Name), food.Key(f.Brand), now); err != nil {
		return shareMember{}, false, err
	}
	_, err = insertSeat(ctx, tx, share, m, to.Food, true)
	return to, true, err
}

// householdCopies returns the live members of share in userID's
// household but userID, with their food of it.
func householdCopies(ctx context.Context, tx *sql.Tx, share string, userID uuid.UUID) ([]shareMember, error) {
	mates, err := householdMates(ctx, tx, userID)
	if err != nil || len(mates) == 0 || share == "" {
		return nil, err
	}
	members, err := liveMembers(ctx, tx, share, userID)
	if err != nil {
		return nil, err
	}
	var out []shareMember
	for _, m := range members {
		if mates[m.User] {
			out = append(out, m)
		}
	}
	return out, nil
}

// spreadFood writes userID's food f, as saved, into each other member of
// their household's copy (writeCopy), making the copies they lack (a
// recipe's food of the recipe whole), links them to the catalog's row of
// userID's food, and moves the group's version on.
// It returns the group: "" when userID is in no household.
func spreadFood(ctx context.Context, tx *sql.Tx, userID uuid.UUID, f food.Food, now time.Time) (string, error) {
	share, fresh, err := householdSeats(ctx, tx, userID, f, now)
	if err != nil || share == "" {
		return "", err
	}
	if f.Source == food.Recipe && len(fresh) > 0 {
		r, err := recipeInTx(ctx, tx, userID, f.ID)
		if err != nil {
			return "", err
		}
		for _, m := range fresh {
			own, err := foodInTx(ctx, tx, m.User, m.Food)
			if err != nil {
				return "", err
			}
			if err := writeMemberRecipe(ctx, tx, r, userID, m, own, now); err != nil {
				return "", err
			}
		}
	}
	copies, err := householdCopies(ctx, tx, share, userID)
	if err != nil {
		return "", err
	}
	for _, m := range copies {
		if err := writeCopy(ctx, tx, f, m, now); err != nil {
			return "", err
		}
	}
	if err := linkCopies(ctx, tx, userID, f.ID, copies); err != nil {
		return "", err
	}
	if _, err := bumpShare(ctx, tx, share); err != nil {
		return "", err
	}
	return share, nil
}

// writeCopy writes the shared fields of f, a member's food, into member
// m's copy of it, m's note, aliases and archived kept. A name and brand,
// or a barcode, another food of m's has is ErrShareNameTaken, or
// ErrShareBarcodeTaken: which food is theirs alone to know.
func writeCopy(ctx context.Context, tx *sql.Tx, f food.Food, m shareMember, now time.Time) error {
	own, err := foodInTx(ctx, tx, m.User, m.Food)
	if err != nil {
		return err
	}
	c := f
	c.ID, c.CopyOf = m.Food, uuid.Nil()
	c.Note, c.Aliases, c.Archived = own.Note, own.Aliases, own.Archived
	if err := checkMemberName(ctx, tx, m, c); err != nil {
		return err
	}
	if err := checkMemberBarcodes(ctx, tx, m, c.Barcodes); err != nil {
		return err
	}
	if err := updateFood(ctx, tx, m.User, c, food.Key(c.Name), food.Key(c.Brand), now); err != nil {
		return err
	}
	return replaceFoodChildren(ctx, tx, m.User, c)
}

// checkMemberName refuses f's name and brand for member m's food when
// another food of theirs has them (ErrShareNameTaken).
func checkMemberName(ctx context.Context, tx *sql.Tx, m shareMember, f food.Food) error {
	var taken bool
	err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM foods WHERE user_id = ? AND name_key = ? AND brand_key = ? AND id <> ?)",
		m.User.String(), food.Key(f.Name), food.Key(f.Brand), m.Food.String()).Scan(&taken)
	if err == nil && taken {
		return ErrShareNameTaken
	}
	return err
}

// checkMemberBarcodes refuses codes for member m's food when another food
// of theirs has one (ErrShareBarcodeTaken).
func checkMemberBarcodes(ctx context.Context, tx *sql.Tx, m shareMember, codes []string) error {
	for _, code := range codes {
		var taken bool
		if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM food_barcodes WHERE user_id = ? AND code = ? AND food_id <> ?)",
			m.User.String(), code, m.Food.String()).Scan(&taken); err != nil {
			return err
		}
		if taken {
			return ErrShareBarcodeTaken
		}
	}
	return nil
}

// linkCopies links each of copies to the catalog's row userID's food id
// names, or to none: a household's copies of a food are one product.
func linkCopies(ctx context.Context, tx *sql.Tx, userID, id uuid.UUID, copies []shareMember) error {
	for _, m := range copies {
		if _, err := tx.ExecContext(ctx, `UPDATE foods SET catalog_id = (SELECT catalog_id FROM foods WHERE user_id = ? AND id = ?)
			WHERE user_id = ? AND id = ?`, userID.String(), id.String(), m.User.String(), m.Food.String()); err != nil {
			return err
		}
	}
	return nil
}

// unlinkCopies links copies to no catalog row, before an undo puts back
// the row of the saver's food: a row the save made goes when no food
// names it, and linkCopies then links the copies as the saver's food is.
func unlinkCopies(ctx context.Context, tx *sql.Tx, copies []shareMember) error {
	for _, m := range copies {
		if _, err := tx.ExecContext(ctx, "UPDATE foods SET catalog_id = NULL WHERE user_id = ? AND id = ?",
			m.User.String(), m.Food.String()); err != nil {
			return err
		}
	}
	return nil
}

// householdVersion is the version of the group of userID's food id while
// it is a household's (another live member of it is of userID's
// household, by a household seat or an explicit share's from before the
// household), which a save of
// the food records with it: an undo after another member's save, which
// moved it on, finds the food changed since though its own fields came
// back the same. 0 for a food in no such group.
func householdVersion(ctx context.Context, tx *sql.Tx, userID, id uuid.UUID) (int64, error) {
	var v int64
	err := tx.QueryRowContext(ctx, `SELECT (SELECT max(g.version) FROM food_shares AS g WHERE g.share_id = mine.share_id)
		FROM food_shares AS mine
		WHERE mine.user_id = ? AND mine.food_id = ? AND mine.left_at IS NULL
			AND EXISTS (SELECT 1 FROM food_shares AS h
				JOIN household_members AS mate ON mate.user_id = h.user_id
				JOIN household_members AS me ON me.household_id = mate.household_id AND me.user_id = mine.user_id
				WHERE h.share_id = mine.share_id AND h.user_id <> mine.user_id AND h.left_at IS NULL)`,
		userID.String(), id.String()).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return v, err
}

// foodState is a food as the events of its saves record it, with its
// household group's version (householdVersion; none outside one): the
// JSON of the food alone then, as before households.
type foodState struct {
	food.Food
	ShareVersion int64 `json:"share_version,omitempty"`
}

// takeBackCopies takes away each other household member's copy of
// userID's food id, of a save that made it, which undo takes back: a copy
// goes, unless a meal or a recipe of theirs uses it, which keeps it,
// archived and of no catalog row.
func takeBackCopies(ctx context.Context, tx *sql.Tx, userID, id uuid.UUID, now time.Time) error {
	share, err := groupOf(ctx, tx, userID, id)
	if err != nil || share == "" {
		return err
	}
	copies, err := householdCopies(ctx, tx, share, userID)
	if err != nil {
		return err
	}
	for _, m := range copies {
		used, err := foodInUse(ctx, tx, m.User, m.Food)
		if err != nil {
			return err
		}
		if !used {
			if _, err := tx.ExecContext(ctx, "DELETE FROM foods WHERE user_id = ? AND id = ?", m.User.String(), m.Food.String()); err != nil {
				return err
			}
			continue
		}
		if err := setArchived(ctx, tx, m.User, m.Food, true, now); err != nil {
			return err
		}
		if err := unlinkCopies(ctx, tx, []shareMember{m}); err != nil {
			return err
		}
	}
	return nil
}

// sharedOutside reports whether userID's food id is in a group with a
// live member outside their household: one an explicit share (403) gave
// it, whose recipe would stay of no one's making if undo took it away.
func sharedOutside(ctx context.Context, tx *sql.Tx, userID, id uuid.UUID) (bool, error) {
	share, _, err := shareOf(ctx, tx, userID, id)
	if err != nil || share == "" {
		return false, err
	}
	others, err := liveMembers(ctx, tx, share, userID)
	if err != nil {
		return false, err
	}
	mates, err := householdMates(ctx, tx, userID)
	if err != nil {
		return false, err
	}
	for _, m := range others {
		if !mates[m.User] {
			return true, nil
		}
	}
	return false, nil
}

// ownIngredients makes each ingredient of items of a food of a member of
// to's household (FoodUser) name to's own copy of it: a household's
// recipes are made of each member's own foods. A food with no copy of
// to's yet gets its copies first (copyIn); one of none (its owner left
// it) stays the owner's, as a shared recipe's ingredient of another
// member's food is.
func ownIngredients(ctx context.Context, tx *sql.Tx, items []meal.Item, to uuid.UUID, now time.Time) error {
	mates, err := householdMates(ctx, tx, to)
	if err != nil || len(mates) == 0 {
		return err
	}
	for i := range items {
		it := &items[i]
		if it.FoodID == uuid.Nil() || !mates[it.FoodUser] {
			continue
		}
		c, err := copyIn(ctx, tx, it.FoodUser, it.FoodID, to, now)
		if err != nil {
			return err
		}
		if c != uuid.Nil() {
			it.FoodID, it.FoodUser = c, uuid.Nil()
		}
	}
	return nil
}

// copyIn returns the id of to's copy of owner's food id, a member of
// their household's, making the household's copies of it when there are
// none (a food of before the household); uuid.Nil when there is none to
// make: the food is gone, or its owner left it.
func copyIn(ctx context.Context, tx *sql.Tx, owner, id, to uuid.UUID, now time.Time) (uuid.UUID, error) {
	c, err := copyOf(ctx, tx, owner, id, to)
	if err != nil || c != uuid.Nil() {
		return c, err
	}
	f, err := foodInTx(ctx, tx, owner, id)
	if errors.Is(err, ErrNotFound) {
		return uuid.Nil(), nil
	}
	if err != nil {
		return uuid.Nil(), err
	}
	if f.Source == food.Recipe {
		err = spreadRecipe(ctx, tx, owner, f, now)
	} else {
		_, err = spreadFood(ctx, tx, owner, f, now)
	}
	if err != nil {
		return uuid.Nil(), err
	}
	return copyOf(ctx, tx, owner, id, to)
}

// copyOf returns the id of to's live food in the group of owner's food
// id, or uuid.Nil.
func copyOf(ctx context.Context, tx *sql.Tx, owner, id, to uuid.UUID) (uuid.UUID, error) {
	var s string
	err := tx.QueryRowContext(ctx, `SELECT theirs.food_id FROM food_shares AS mine
		JOIN food_shares AS theirs ON theirs.share_id = mine.share_id AND theirs.user_id = ?3 AND theirs.left_at IS NULL
		WHERE mine.user_id = ?1 AND mine.food_id = ?2 AND mine.left_at IS NULL`,
		owner.String(), id.String(), to.String()).Scan(&s)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil(), nil
	}
	if err != nil {
		return uuid.Nil(), err
	}
	return uuid.Parse(s)
}

// spreadRecipe gives each other member of userID's household their copy
// of userID's recipe of food f, as it is, and writes it into every member
// of its group (syncShared).
func spreadRecipe(ctx context.Context, tx *sql.Tx, userID uuid.UUID, f food.Food, now time.Time) error {
	if _, _, err := householdSeats(ctx, tx, userID, f, now); err != nil {
		return err
	}
	r, err := recipeInTx(ctx, tx, userID, f.ID)
	if err != nil {
		return err
	}
	return syncShared(ctx, tx, userID, f.ID, r, now)
}

// householdOf returns the id of userID's household, or "".
func householdOf(ctx context.Context, tx *sql.Tx, userID uuid.UUID) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, "SELECT household_id FROM household_members WHERE user_id = ?", userID.String()).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// groupOf is shareOf's group of userID's food id, "" for a food in none
// or one they left: whose copies are none of theirs to write.
func groupOf(ctx context.Context, tx *sql.Tx, userID, id uuid.UUID) (string, error) {
	share, _, err := shareOf(ctx, tx, userID, id)
	if errors.Is(err, ErrNotFound) {
		return "", nil
	}
	return share, err
}
