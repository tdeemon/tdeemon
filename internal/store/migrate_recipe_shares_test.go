package store

import (
	"testing"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/event"
	"github.com/zarnautovic/tdeemon/internal/domain/food"
)

// 0032 rebuilds recipe_items with the owner of each ingredient's food, the
// recipe's user for every row before it, and the events for a share and
// its stop: a recipe reads as it was, and an ingredient of another
// user's food keeps its values when that user goes.
func TestMigrationOfRecipeShares(t *testing.T) {
	t.Parallel()

	db := openEmpty(t)
	if err := db.migrate(t.Context(), upTo(t, "0031_food_catalog_search.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0031: %v", err)
	}
	must := func(q string, args ...any) {
		t.Helper()
		if _, err := db.write.ExecContext(t.Context(), q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	ana, ivo := uuid.MustParse("0199f0a0-0000-7000-8000-000000000001"), uuid.MustParse("0199f0a0-0000-7000-8000-000000000002")
	for _, u := range []struct {
		id    uuid.UUID
		email string
	}{{ana, "ana@example.com"}, {ivo, "ivo@example.com"}} {
		must(`INSERT INTO users (id, email, name, role, locale, timezone, day_start_hour, created_at, updated_at)
			VALUES (?, ?, 'X', 'user', 'en', 'UTC', 4, '2026-11-01T08:00:00Z', '2026-11-01T08:00:00Z')`, u.id.String(), u.email)
	}
	now := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	actor := event.Actor{Kind: event.User, Client: "web"}
	beef := food.Food{
		ID: uuid.NewV7(), Name: "Beef mince", Basis: food.Per100g, Source: food.Label,
		Nutrients: food.Nutrients{Kcal: 250, Protein: 26, Carbs: 0, Fat: 17},
	}
	if _, err := db.SaveFood(t.Context(), ana, actor, beef, now); err != nil {
		t.Fatal(err)
	}
	sauce := uuid.NewV7()
	must(`INSERT INTO foods (id, user_id, name, name_key, basis, kcal, protein, carbs, fat, source, created_at, updated_at)
		VALUES (?, ?, 'Bolognese', 'bolognese', '100g', 150, 13, 3, 10, 'recipe', ?, ?)`, sauce.String(), ana.String(), formatTime(now), formatTime(now))
	must(`INSERT INTO recipes (food_id, user_id, yield_g) VALUES (?, ?, 600)`, sauce.String(), ana.String())
	must(`INSERT INTO recipe_items (recipe_id, user_id, position, food_id, name, quantity_amount, quantity_unit, amount, unit,
		kcal, protein, carbs, fat, source) VALUES (?, ?, 0, ?, 'Beef mince', 500, 'g', 500, 'g', 1250, 130, 0, 85, 'label')`,
		sauce.String(), ana.String(), beef.ID.String())

	if err := db.migrate(t.Context(), upTo(t, "0032_recipe_shares.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0032: %v", err)
	}
	var owner string
	if err := db.write.QueryRowContext(t.Context(), "SELECT food_user_id FROM recipe_items WHERE recipe_id = ?", sauce.String()).Scan(&owner); err != nil || owner != ana.String() {
		t.Errorf("the ingredient's owner = %q, %v; want Ana", owner, err)
	}
	must(`INSERT INTO events (id, user_id, at, actor, client, operation, entity) VALUES (?, ?, ?, 'user', 'web', 'recipe.share', ?)`,
		uuid.NewV7().String(), ana.String(), formatTime(now), sauce.String())
	// The store reads the share groups of 0033, food_shares.
	if err := db.migrate(t.Context(), upTo(t, "0033_food_shares.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0033: %v", err)
	}
	r, err := db.RecipeByID(t.Context(), ana, sauce)
	if err != nil || len(r.Items) != 1 || r.Items[0].FoodID != beef.ID || r.Items[0].FoodUser != uuid.Nil() || r.Items[0].Nutrients.Kcal != 1250 {
		t.Fatalf("Ana's recipe after 0032 = %+v, %v; want her beef, hers, 1250 kcal", r, err)
	}
	ingredientOfAFoodGone(t, db, ana, ivo, beef.ID, now)
}

// ingredientOfAFoodGone gives ivo a recipe of ana's beef, as a share
// would make it, and deletes ana straight in the table, past DeleteUser,
// which would give Ivo a food of his own of it first: the foreign key
// alone leaves the ingredient its values, of no food.
func ingredientOfAFoodGone(t *testing.T, db *DB, ana, ivo, beef uuid.UUID, now time.Time) {
	t.Helper()
	must := func(q string, args ...any) {
		t.Helper()
		if _, err := db.write.ExecContext(t.Context(), q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	ivos := uuid.NewV7()
	must(`INSERT INTO foods (id, user_id, name, name_key, basis, kcal, protein, carbs, fat, source, created_at, updated_at)
		VALUES (?, ?, 'Bolognese', 'bolognese', '100g', 150, 13, 3, 10, 'recipe', ?, ?)`, ivos.String(), ivo.String(), formatTime(now), formatTime(now))
	must(`INSERT INTO recipes (food_id, user_id, yield_g) VALUES (?, ?, 600)`, ivos.String(), ivo.String())
	must(`INSERT INTO recipe_items (recipe_id, user_id, position, food_user_id, food_id, name, quantity_amount, quantity_unit,
		amount, unit, kcal, protein, carbs, fat, source) VALUES (?, ?, 0, ?, ?, 'Beef mince', 500, 'g', 500, 'g', 1250, 130, 0, 85, 'label')`,
		ivos.String(), ivo.String(), ana.String(), beef.String())
	must(`DELETE FROM users WHERE id = ?`, ana.String())
	if r, err := db.RecipeByID(t.Context(), ivo, ivos); err != nil || len(r.Items) != 1 || r.Items[0].FoodID != uuid.Nil() || r.Items[0].Nutrients.Kcal != 1250 {
		t.Errorf("Ivo's recipe after Ana went = %+v, %v; want the beef's values, of no food", r, err)
	}
}
