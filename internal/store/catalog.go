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
)

// The catalog is the instance's foods, no user's: every user's declared
// foods (a label's, Open Food Facts', a food table's) come to one row a
// product, which every user's search finds. A row is kept in step with
// the latest save of a user's food that names it (foods.catalog_id): the
// last values win. A barcode stays the product's that had it first. A
// user's notes and aliases stay theirs, and a user's deletion leaves
// the catalog.

// catalogued reports whether f's values go into the catalog: a food in
// use, of declared values.
func catalogued(f food.Food) bool {
	if f.Archived {
		return false
	}
	switch f.Source {
	case food.Label, food.Reference, food.OpenFoodFacts:
		return true
	case food.Estimate, food.User, food.Dish, food.Recipe:
	}
	return false
}

// catalogState is a catalog row as a save found it, for the save's
// undo: its food and its version.
type catalogState struct {
	Food    food.Food `json:"food"`
	Version int64     `json:"version"`
}

// syncCatalog puts userID's food f, as its save of event saved it, into
// the catalog, unless it is not catalogued, links f to its row, and
// records what it did for the save's undo (catalog_saves). The row is
// the product of f's name and brand; else the row f names already,
// renamed to f's; else the row of one of f's barcodes; else a new one.
// Its values and servings become f's, and f's barcodes its, but those
// another row has: a barcode stays the product's that had it first.
func syncCatalog(ctx context.Context, tx *sql.Tx, userID uuid.UUID, f food.Food, saved uuid.UUID, now time.Time) error {
	if !catalogued(f) || saved == uuid.Nil() {
		return nil
	}
	nameKey, brandKey := food.Key(f.Name), food.Key(f.Brand)
	link, err := foodCatalogID(ctx, tx, userID, f.ID)
	if err != nil {
		return err
	}
	target, err := catalogTarget(ctx, tx, userID, f, nameKey, brandKey)
	if err != nil {
		return err
	}
	var before *catalogState
	version := int64(1)
	if target.exists {
		st, err := catalogStateInTx(ctx, tx, target.id)
		if err != nil {
			return err
		}
		before, version = &st, st.Version+1
	}
	if err := writeCatalogFood(ctx, tx, target, f, version, now); err != nil {
		return err
	}
	if err := writeCatalogCodes(ctx, tx, target.id, f.Barcodes); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE foods SET catalog_id = ? WHERE user_id = ? AND id = ?",
		target.id, userID.String(), f.ID.String()); err != nil {
		return err
	}
	state, err := stateJSON(before)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO catalog_saves (user_id, event_id, catalog_id, version, before_state, link_before)
		VALUES (?, ?, ?, ?, ?, ?)`, userID.String(), saved.String(), target.id, version, state, link)
	return err
}

// restoreCatalog takes back what userID's save of food e did to the
// catalog, after the food is put back (or gone): the food names its row
// before again, and the row, unless a save came since, which it leaves,
// is put back as it was; or a row the save made goes, when no food
// names it. A save before the catalog, which recorded nothing of it, is
// restoreLegacyCatalog's.
func restoreCatalog(ctx context.Context, tx *sql.Tx, userID uuid.UUID, e event.Event, now time.Time) error {
	var (
		id      string
		version int64
		state   sql.NullString
		link    sql.NullString
	)
	err := tx.QueryRowContext(ctx, "SELECT catalog_id, version, before_state, link_before FROM catalog_saves WHERE user_id = ? AND event_id = ?",
		userID.String(), e.ID.String()).Scan(&id, &version, &state, &link)
	if errors.Is(err, sql.ErrNoRows) {
		return restoreLegacyCatalog(ctx, tx, userID, e, now)
	}
	if err != nil {
		return err
	}
	// The row before, unless it went since.
	if _, err := tx.ExecContext(ctx, "UPDATE foods SET catalog_id = (SELECT id FROM catalog_foods WHERE id = ?) WHERE user_id = ? AND id = ?",
		link, userID.String(), e.Entity); err != nil {
		return err
	}
	var current int64
	err = tx.QueryRowContext(ctx, "SELECT version FROM catalog_foods WHERE id = ?", id).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && current != version) {
		return nil
	}
	if err != nil {
		return err
	}
	if !state.Valid {
		_, err := tx.ExecContext(ctx, "DELETE FROM catalog_foods WHERE id = ?1 AND NOT EXISTS (SELECT 1 FROM foods WHERE catalog_id = ?1)", id)
		return err
	}
	var before catalogState
	if err := decode([]byte(state.String), &before); err != nil {
		return err
	}
	return putBackCatalogRow(ctx, tx, id, before, now)
}

// restoreLegacyCatalog takes back what userID's save of food e, made
// before the catalog, gave it through the migration, which made a row
// of the food saved last, and of its time: the row of the name and brand
// the save left, untouched since (its first version) and made of this
// save (its time e's). A save that made the food, or made a food the
// catalog does not take one it does, takes its row along when no other
// food names it; any other gets the food's values before back, and its
// barcodes, but those of the save's own, leaving another's; and the row
// takes the time of the food's save before, whose undo it is then of.
// Anything else stays, and a save of a food the catalog does not take
// never changes it.
func restoreLegacyCatalog(ctx context.Context, tx *sql.Tx, userID uuid.UUID, e event.Event, now time.Time) error {
	id, before, after, err := legacyRow(ctx, tx, e)
	if err != nil || id == "" {
		return err
	}
	if before == nil || !catalogued(*before) {
		if _, err := tx.ExecContext(ctx, "UPDATE foods SET catalog_id = NULL WHERE user_id = ? AND id = ?", userID.String(), e.Entity); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "DELETE FROM catalog_foods WHERE id = ?1 AND NOT EXISTS (SELECT 1 FROM foods WHERE catalog_id = ?1)", id)
		return err
	}
	var taken bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM catalog_foods WHERE name_key = ? AND brand_key = ? AND id <> ?)",
		food.Key(before.Name), food.Key(before.Brand), id).Scan(&taken); err != nil || taken {
		return err
	}
	if err := writeCatalogFood(ctx, tx, catalogRow{id, true}, *before, 1, now); err != nil {
		return err
	}
	if err := putBackLegacyCodes(ctx, tx, id, before.Barcodes, after.Barcodes); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE catalog_foods SET updated_at = coalesce((SELECT max(at) FROM events
		WHERE user_id = ?1 AND entity = ?2 AND operation = 'food.save' AND undone_by IS NULL AND id <> ?3 AND at <= ?4), updated_at)
		WHERE id = ?5`, userID.String(), e.Entity, e.ID.String(), formatTime(e.At), id)
	return err
}

// legacyRow returns the row the migration made of the food's save e, a
// catalogued food's, as restoreLegacyCatalog says, with the food before
// (nil for none) and after e; or "" for none.
func legacyRow(ctx context.Context, tx *sql.Tx, e event.Event) (string, *food.Food, food.Food, error) {
	var after food.Food
	if e.Action != event.FoodSave || e.After == nil {
		return "", nil, after, nil
	}
	if err := decode(e.After, &after); err != nil || !catalogued(after) {
		return "", nil, after, err
	}
	// By the name and brand the save left, as the food may be gone.
	var id string
	err := tx.QueryRowContext(ctx, "SELECT id FROM catalog_foods WHERE name_key = ? AND brand_key = ? AND version = 1 AND updated_at = ?",
		food.Key(after.Name), food.Key(after.Brand), formatTime(e.At)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, after, nil
	}
	if err != nil || e.Before == nil {
		return id, nil, after, err
	}
	var before food.Food
	if err := decode(e.Before, &before); err != nil {
		return "", nil, after, err
	}
	return id, &before, after, nil
}

// putBackLegacyCodes takes the barcodes a save brought (in after, not
// before) out of the row id, and gives it back those of before no other
// row has, leaving another's.
func putBackLegacyCodes(ctx context.Context, tx *sql.Tx, id string, before, after []string) error {
	for _, code := range after {
		if !slices.Contains(before, code) {
			if _, err := tx.ExecContext(ctx, "DELETE FROM catalog_barcodes WHERE code = ? AND catalog_id = ?", code, id); err != nil {
				return err
			}
		}
	}
	for _, code := range before {
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO catalog_barcodes (code, catalog_id) VALUES (?, ?)", code, id); err != nil {
			return err
		}
	}
	return nil
}

// putBackCatalogRow puts the catalog's row id back as before, as a save
// found it, unless another row took its name and brand
// since, whose save stays, and so does this row, as it is. No save into
// the row came since, so its barcodes are its own before, but those the
// save let go, and the save's own: the save's go, and its own come back
// in their order, but those another row has since, which stay that
// row's.
func putBackCatalogRow(ctx context.Context, tx *sql.Tx, id string, before catalogState, now time.Time) error {
	var taken bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM catalog_foods WHERE name_key = ? AND brand_key = ? AND id <> ?)",
		food.Key(before.Food.Name), food.Key(before.Food.Brand), id).Scan(&taken); err != nil || taken {
		return err
	}
	if err := writeCatalogFood(ctx, tx, catalogRow{id, true}, before.Food, before.Version, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM catalog_barcodes WHERE catalog_id = ?", id); err != nil {
		return err
	}
	for _, code := range before.Food.Barcodes {
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO catalog_barcodes (code, catalog_id) VALUES (?, ?)", code, id); err != nil {
			return err
		}
	}
	return nil
}

// catalogRow is the catalog's row a food goes into: its id, and whether
// it is there yet.
type catalogRow struct {
	id     string
	exists bool
}

// catalogTarget finds the row userID's food f goes into, as syncCatalog
// says, or makes a new id for it.
func catalogTarget(ctx context.Context, tx *sql.Tx, userID uuid.UUID, f food.Food, nameKey, brandKey string) (catalogRow, error) {
	var id string
	err := tx.QueryRowContext(ctx, "SELECT id FROM catalog_foods WHERE name_key = ? AND brand_key = ?", nameKey, brandKey).Scan(&id)
	if err == nil {
		return catalogRow{id, true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return catalogRow{}, err
	}
	err = tx.QueryRowContext(ctx, "SELECT c.id FROM foods AS f JOIN catalog_foods AS c ON c.id = f.catalog_id WHERE f.user_id = ? AND f.id = ?",
		userID.String(), f.ID.String()).Scan(&id)
	if err == nil {
		return catalogRow{id, true}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return catalogRow{}, err
	}
	for _, code := range f.Barcodes {
		err := tx.QueryRowContext(ctx, "SELECT catalog_id FROM catalog_barcodes WHERE code = ?", code).Scan(&id)
		if err == nil {
			return catalogRow{id, true}, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return catalogRow{}, err
		}
	}
	return catalogRow{uuid.NewV7().String(), false}, nil
}

// writeCatalogFood writes f's name, values and servings into row at
// version, and its words into the search, but not its barcodes.
func writeCatalogFood(ctx context.Context, tx *sql.Tx, row catalogRow, f food.Food, version int64, now time.Time) error {
	nameKey, brandKey := food.Key(f.Name), food.Key(f.Brand)
	o := optionalNutrients(&f.Nutrients)
	var portion any
	if f.PortionGrams > 0 {
		portion = f.PortionGrams
	}
	values := []any{
		f.Name, f.Brand, nameKey, brandKey, string(f.Basis), f.Nutrients.Kcal, f.Nutrients.Protein, f.Nutrients.Carbs,
		f.Nutrients.Fat, nullable(*o[0]), nullable(*o[1]), nullable(*o[2]), nullable(*o[3]), nullable(*o[4]),
		nullable(*o[5]), portion, string(f.Source), formatTime(now), version,
	}
	var err error
	if row.exists {
		_, err = tx.ExecContext(ctx, `UPDATE catalog_foods SET name = ?, brand = ?, name_key = ?, brand_key = ?, basis = ?,
			kcal = ?, protein = ?, carbs = ?, fat = ?, fiber = ?, saturated_fat = ?, sugars = ?, salt = ?, polyols = ?,
			alcohol = ?, portion_g = ?, source = ?, updated_at = ?, version = ? WHERE id = ?`, append(values, row.id)...)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO catalog_foods (name, brand, name_key, brand_key, basis, kcal, protein,
			carbs, fat, fiber, saturated_fat, sugars, salt, polyols, alcohol, portion_g, source, updated_at, version, id,
			created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			append(values, row.id, formatTime(now))...)
	}
	if err != nil {
		return err
	}
	for _, q := range []string{"DELETE FROM catalog_servings WHERE catalog_id = ?", "DELETE FROM catalog_fts WHERE catalog_id = ?"} {
		if _, err := tx.ExecContext(ctx, q, row.id); err != nil {
			return err
		}
	}
	for _, s := range f.Servings {
		if _, err := tx.ExecContext(ctx, "INSERT INTO catalog_servings (catalog_id, name, name_key, amount) VALUES (?, ?, ?, ?)",
			row.id, s.Name, food.Key(s.Name), s.Amount); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO catalog_fts (catalog_id, name, brand) VALUES (?, ?, ?)",
		row.id, searchText(f.Name), searchText(f.Brand))
	return err
}

// writeCatalogCodes adds codes to the row id's barcodes, but those
// another row has, and leaves the row food.MaxBarcodes at most: codes,
// and the newest of its others.
func writeCatalogCodes(ctx context.Context, tx *sql.Tx, id string, codes []string) error {
	for _, code := range codes {
		// A code of the row's goes and comes anew, so the newest rowids
		// are the codes of the latest saves.
		if _, err := tx.ExecContext(ctx, "DELETE FROM catalog_barcodes WHERE code = ? AND catalog_id = ?", code, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO catalog_barcodes (code, catalog_id) VALUES (?, ?)", code, id); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM catalog_barcodes WHERE catalog_id = ?1 AND rowid NOT IN (
		SELECT rowid FROM catalog_barcodes WHERE catalog_id = ?1 ORDER BY rowid DESC LIMIT ?2)`, id, food.MaxBarcodes)
	return err
}

// catalogStateInTx reads the catalog's row id whole, for a save's undo.
func catalogStateInTx(ctx context.Context, tx *sql.Tx, id string) (catalogState, error) {
	found, err := queryFoods(ctx, tx, "SELECT "+catalogColumns+" FROM catalog_foods AS c WHERE c.id = ?", id)
	if err != nil {
		return catalogState{}, err
	}
	if len(found) == 0 {
		return catalogState{}, ErrNotFound
	}
	st := catalogState{Food: found[0]}
	if err := catalogChildren(ctx, tx, &st.Food); err != nil {
		return catalogState{}, err
	}
	err = tx.QueryRowContext(ctx, "SELECT version FROM catalog_foods WHERE id = ?", id).Scan(&st.Version)
	return st, err
}

// foodCatalogID returns the catalog's row userID's food id names, if any.
func foodCatalogID(ctx context.Context, tx *sql.Tx, userID, id uuid.UUID) (sql.NullString, error) {
	var c sql.NullString
	err := tx.QueryRowContext(ctx, "SELECT catalog_id FROM foods WHERE user_id = ? AND id = ?", userID.String(), id.String()).Scan(&c)
	if errors.Is(err, sql.ErrNoRows) {
		return sql.NullString{}, nil
	}
	return c, err
}

// catalogColumns are a catalog row's, in the shape of foodColumns: no
// note, never archived.
const catalogColumns = `c.id, c.name, c.brand, c.basis, c.kcal, c.protein, c.carbs, c.fat,
	c.fiber, c.saturated_fat, c.sugars, c.salt, c.polyols, c.alcohol, c.portion_g, c.source, '', 0`

// FindCatalog returns up to limit of the catalog's foods that match
// query as FindFoods matches, but the ones userID has already: a food of
// theirs, archived too, that names the row, or has its name and brand,
// or one of its barcodes. Their IDs are the catalog's, no user's food's.
// The order is FindFoods': the name's matches first, then the shorter
// name.
func (db *DB) FindCatalog(ctx context.Context, userID uuid.UUID, query string, limit int) ([]food.Food, error) {
	match := ftsQuery(query)
	if match == "" || limit <= 0 {
		return nil, nil
	}
	limit = min(limit, MaxFoodResults)
	var out []food.Food
	err := db.readTx(ctx, func(tx *sql.Tx) error {
		var err error
		out, err = queryFoods(ctx, tx, `
			SELECT `+catalogColumns+` FROM catalog_foods AS c
			WHERE c.id IN (SELECT catalog_id FROM catalog_fts WHERE catalog_fts MATCH ?2)
				AND NOT EXISTS (SELECT 1 FROM foods AS f WHERE f.user_id = ?1
					AND (f.catalog_id = c.id OR (f.name_key = c.name_key AND f.brand_key = c.brand_key)))
				AND NOT EXISTS (SELECT 1 FROM catalog_barcodes AS b
					JOIN food_barcodes AS fb ON fb.code = b.code AND fb.user_id = ?1
					WHERE b.catalog_id = c.id)
			ORDER BY CASE WHEN c.id IN (SELECT catalog_id FROM catalog_fts WHERE catalog_fts MATCH ?3) THEN 0 ELSE 1 END,
				length(c.name), c.name, c.brand
			LIMIT ?4`, userID.String(), match, "{name} : ("+match+")", limit)
		if err != nil {
			return err
		}
		for i := range out {
			if err := catalogChildren(ctx, tx, &out[i]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("find catalog: %w", err)
	}
	return out, nil
}

// CatalogFood returns the catalog's food with id, or ErrNotFound.
func (db *DB) CatalogFood(ctx context.Context, id uuid.UUID) (food.Food, error) {
	return db.catalogFood(ctx, "c.id = ?", id.String())
}

// CatalogFoodByBarcode returns the catalog's food of the barcode code,
// in its canonical form (food.Barcode), or ErrNotFound.
func (db *DB) CatalogFoodByBarcode(ctx context.Context, code string) (food.Food, error) {
	return db.catalogFood(ctx, "c.id = (SELECT catalog_id FROM catalog_barcodes WHERE code = ?)", code)
}

func (db *DB) catalogFood(ctx context.Context, where, arg string) (food.Food, error) {
	var f food.Food
	err := db.readTx(ctx, func(tx *sql.Tx) error {
		found, err := queryFoods(ctx, tx, "SELECT "+catalogColumns+" FROM catalog_foods AS c WHERE "+where, arg)
		if err != nil {
			return err
		}
		if len(found) == 0 {
			return ErrNotFound
		}
		f = found[0]
		return catalogChildren(ctx, tx, &f)
	})
	if errors.Is(err, ErrNotFound) {
		return food.Food{}, err
	}
	if err != nil {
		return food.Food{}, fmt.Errorf("catalog food: %w", err)
	}
	return f, nil
}

// catalogChildren reads the servings and barcodes of the catalog's food
// f, in the order written, food.MaxBarcodes of them at most.
func catalogChildren(ctx context.Context, tx *sql.Tx, f *food.Food) error {
	id := f.ID.String()
	rows, err := tx.QueryContext(ctx, "SELECT name, amount FROM catalog_servings WHERE catalog_id = ? ORDER BY rowid", id)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var s food.Serving
		if err := rows.Scan(&s.Name, &s.Amount); err != nil {
			return err
		}
		f.Servings = append(f.Servings, s)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// The newest, as a food takes no more; a row the migration made may
	// have more.
	brows, err := tx.QueryContext(ctx, `SELECT code FROM (SELECT code, rowid AS r FROM catalog_barcodes WHERE catalog_id = ?
		ORDER BY rowid DESC LIMIT ?) ORDER BY r`, id, food.MaxBarcodes)
	if err != nil {
		return err
	}
	defer func() { _ = brows.Close() }()
	for brows.Next() {
		var code string
		if err := brows.Scan(&code); err != nil {
			return err
		}
		f.Barcodes = append(f.Barcodes, code)
	}
	return brows.Err()
}
