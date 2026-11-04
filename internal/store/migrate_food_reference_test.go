package store

import (
	"testing"
	"time"
	"uuid"

	"github.com/tdeemon/tdeemon/internal/domain/event"
	"github.com/tdeemon/tdeemon/internal/domain/food"
)

// 0029 rebuilds foods, meal_items and recipe_items for the source
// reference: every row stays as it was, under its rowid, the search finds
// the foods still, a food's delete takes its words out of the index, and
// the source reference is taken where it was refused before.
func TestMigrationOfTheReferenceSourceKeepsTheRows(t *testing.T) {
	t.Parallel()

	db := openEmpty(t)
	if err := db.migrate(t.Context(), upTo(t, "0028_invites.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0028: %v", err)
	}
	user, ids := seedBeforeReference(t, db)
	exec := func(q string, args ...any) error {
		_, err := db.write.ExecContext(t.Context(), q, args...)
		return err
	}
	must := func(q string, args ...any) {
		t.Helper()
		if err := exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	before := sourcesDump(t, db)
	if err := exec(`UPDATE foods SET source = 'reference' WHERE id = ?`, ids["Jabuka"]); err == nil {
		t.Fatal("before 0029, the source reference is taken")
	}

	if err := db.migrate(t.Context(), upTo(t, "0029_food_reference.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0029: %v", err)
	}
	if after := sourcesDump(t, db); after != before {
		t.Errorf("the rows after the migration:\n%s\nwant\n%s", after, before)
	}
	// Every migration since too: the search runs on the newest schema.
	if err := db.migrate(t.Context(), upTo(t, "9999"), discard()); err != nil {
		t.Fatalf("migrate from 0029: %v", err)
	}
	if found, err := db.FindFoods(t.Context(), user, "appl", 10); err != nil || len(found) != 1 || found[0].Name != "Jabuka" {
		t.Errorf(`FindFoods("appl") = %v, %v; want Jabuka by its alias`, found, err)
	}
	for _, q := range []string{
		`UPDATE foods SET source = 'reference' WHERE id = ?`,
		`UPDATE meal_items SET source = 'reference' WHERE food_id = ?`,
		`UPDATE recipe_items SET source = 'reference' WHERE food_id = ?`,
	} {
		if err := exec(q, ids["Jabuka"]); err != nil {
			t.Errorf("%s: %v", q, err)
		}
	}
	if err := exec(`UPDATE foods SET source = 'guess' WHERE id = ?`, ids["Jabuka"]); err == nil {
		t.Error("after 0029, any source is taken")
	}
	must(`DELETE FROM meal_items WHERE food_id = ?`, ids["Tuna u vlastitom soku"])
	must(`DELETE FROM foods WHERE id = ?`, ids["Tuna u vlastitom soku"])
	var left int
	if err := db.write.QueryRowContext(t.Context(), "SELECT count(*) FROM foods_fts WHERE food_id = ?",
		ids["Tuna u vlastitom soku"]).Scan(&left); err != nil || left != 0 {
		t.Errorf("a deleted food's rows in the index = %d, %v; want none", left, err)
	}
}

// seedBeforeReference makes, before 0029, a user's foods (an estimate
// with an alias, a user's, a recipe's), a meal of their items and a
// one-off estimate, and the recipe of an ingredient; it returns the user
// and the foods' ids by name.
func seedBeforeReference(t *testing.T, db *DB) (uuid.UUID, map[string]string) {
	t.Helper()
	must := func(q string, args ...any) {
		t.Helper()
		if _, err := db.write.ExecContext(t.Context(), q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	user := uuid.MustParse("0199f0a0-0000-7000-8000-000000000001")
	must(`INSERT INTO users (id, email, name, role, locale, timezone, day_start_hour, created_at, updated_at)
		VALUES (?, 'ana@example.com', 'Ana', 'user', 'en', 'UTC', 4, '2026-11-01T08:00:00Z', '2026-11-01T08:00:00Z')`, user.String())
	now := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	actor := event.Actor{Kind: event.User, Client: "web"}
	ids := map[string]string{}
	foods := []food.Food{
		{Name: "Jabuka", Source: food.Estimate, Aliases: []string{"apple"}},
		{Name: "Tuna u vlastitom soku", Brand: "Eva", Source: food.User},
	}
	householdStandIns(t, db)
	for i := range foods {
		f := &foods[i]
		f.ID = uuid.NewV7()
		f.Basis = food.Per100g
		f.Nutrients = food.Nutrients{Kcal: 52, Protein: 0.3, Carbs: 14, Fat: 0.2}
		if _, err := db.SaveFood(t.Context(), user, actor, *f, now); err != nil {
			t.Fatal(err)
		}
		ids[f.Name] = f.ID.String()
	}
	dropHouseholdStandIns(t, db)
	ids["Gulaš"] = uuid.NewV7().String()
	must(`INSERT INTO foods (id, user_id, name, name_key, basis, kcal, protein, carbs, fat, source, created_at, updated_at)
		VALUES (?, ?, 'Gulaš', 'gulas', '100g', 120, 10, 5, 6, 'recipe', '2026-11-01T08:00:00Z', '2026-11-01T08:00:00Z')`,
		ids["Gulaš"], user.String())
	const meal = "0199f0a0-0000-7000-8000-0000000000a1"
	must(`INSERT INTO meals (id, user_id, eaten_at, day, description, client, note, fingerprint, created_at, updated_at)
		VALUES (?, ?, '2026-11-01T08:00:00Z', '2026-11-01', '', 'web', '', printf('%064d', 1), '2026-11-01T08:00:00Z', '2026-11-01T08:00:00Z')`,
		meal, user.String())
	for i, it := range []struct {
		food         any
		name, source string
	}{
		{ids["Jabuka"], "Jabuka", "estimate"},
		{ids["Tuna u vlastitom soku"], "Tuna u vlastitom soku", "label"},
		{nil, "Plate of the canteen", "estimate"},
	} {
		must(`INSERT INTO meal_items (id, meal_id, user_id, position, food_id, name, amount, unit, kcal, protein, carbs, fat, source, brand)
			VALUES (printf('0199f0a0-0000-7000-8000-0000000000b%d', ?), ?, ?, ?, ?, ?, 100, 'g', 52, 0.3, 14, 0.2, ?, '')`,
			i, meal, user.String(), i, it.food, it.name, it.source)
	}
	must(`INSERT INTO recipes (food_id, user_id, yield_g) VALUES (?, ?, 900)`, ids["Gulaš"], user.String())
	must(`INSERT INTO recipe_items (recipe_id, user_id, position, food_id, name, amount, unit, kcal, protein, carbs, fat, source)
		VALUES (?, ?, 0, ?, 'Jabuka', 200, 'g', 104, 0.6, 28, 0.4, 'estimate')`, ids["Gulaš"], user.String(), ids["Jabuka"])
	return user, ids
}

// sourcesDump is every food, item and ingredient, its rowid, id, name and
// source, a line each.
func sourcesDump(t *testing.T, db *DB) string {
	t.Helper()
	var out string
	if err := db.write.QueryRowContext(t.Context(), `SELECT group_concat(r, char(10)) FROM (
		SELECT 'food ' || rowid || ' ' || id || ' ' || name || ' ' || source AS r FROM foods
		UNION ALL SELECT 'item ' || rowid || ' ' || id || ' ' || ifnull(food_id, '-') || ' ' || name || ' ' || source FROM meal_items
		UNION ALL SELECT 'ingredient ' || rowid || ' ' || food_id || ' ' || name || ' ' || source FROM recipe_items
		ORDER BY 1)`).Scan(&out); err != nil {
		t.Fatal(err)
	}
	return out
}
