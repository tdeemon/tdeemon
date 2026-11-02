package store

import (
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/event"
	"github.com/zarnautovic/tdeemon/internal/domain/food"
)

// catalogSeed is a food saved before the catalog.
type catalogSeed struct {
	id, user, name, key, source, updated string
	kcal                                 float64
	archived                             int
}

// catalogUsers are the users of TestMigrationOfTheCatalog, by name.
var catalogUsers = map[string]string{
	"Ana": "0199f0a0-0000-7000-8000-000000000001",
	"Ivo": "0199f0a0-0000-7000-8000-000000000002",
	"Eva": "0199f0a0-0000-7000-8000-000000000003",
}

// catalogSeeds are their foods: Ana's and Ivo's grapes, one name in two
// cases, Ivo's saved last; Ivo's apple of a food table; Ana's estimate
// and her archived crackers.
var catalogSeeds = []catalogSeed{
	{"0199f0a0-0000-7000-8000-0000000000f1", "Ana", "Bijelo grožđe", "bijelo grožđe", "label", "2026-10-01T08:00:00Z", 69, 0},
	{"0199f0a0-0000-7000-8000-0000000000f2", "Ivo", "BIJELO GROŽĐE", "bijelo grožđe", "label", "2026-10-20T08:00:00Z", 70, 0},
	{"0199f0a0-0000-7000-8000-0000000000f3", "Ivo", "Jabuka", "jabuka", "reference", "2026-10-02T08:00:00Z", 52, 0},
	{"0199f0a0-0000-7000-8000-0000000000f4", "Ana", "Gulaš iz menze", "gulaš iz menze", "estimate", "2026-10-03T08:00:00Z", 120, 0},
	{"0199f0a0-0000-7000-8000-0000000000f5", "Ana", "Krekeri", "krekeri", "label", "2026-10-04T08:00:00Z", 430, 1},
}

// anasCodes are the barcodes of Ana's grapes.
var anasCodes = []string{
	"3850104000010", "4000000000006", "4000000000013", "4000000000020", "4000000000037",
	"4000000000044", "4000000000051", "4000000000068", "4000000000075", "4000000000082",
}

// 0030 and 0031 make the catalog of the declared foods in use saved
// before them: one row a name and brand, of every user's, with the
// values and servings of the one saved last under an id of its own, the
// barcodes of all of them, each food linked to its row, and the rows'
// words in the search, a đ as d and dj too. Estimates and archived foods
// stay out.
func TestMigrationOfTheCatalog(t *testing.T) {
	t.Parallel()

	db := openEmpty(t)
	if err := db.migrate(t.Context(), upTo(t, "0029_food_reference.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0029: %v", err)
	}
	seedCatalog(t, db)
	if err := db.migrate(t.Context(), upTo(t, "0031_food_catalog_search.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0031: %v", err)
	}
	grape := checkCatalogRows(t, db)
	checkCatalogLinks(t, db, grape)
	checkCatalogSearch(t, db, grape)
}

// seedCatalog writes catalogUsers and catalogSeeds, the grapes with a
// barcode and a serving each.
func seedCatalog(t *testing.T, db *DB) {
	t.Helper()
	must := func(q string, args ...any) {
		t.Helper()
		if _, err := db.write.ExecContext(t.Context(), q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	for name, id := range catalogUsers {
		must(`INSERT INTO users (id, email, name, role, locale, timezone, day_start_hour, created_at, updated_at)
			VALUES (?, lower(?) || '@example.com', ?, 'user', 'en', 'UTC', 4, '2026-10-01T08:00:00Z', '2026-10-01T08:00:00Z')`,
			id, name, name)
	}
	for _, f := range catalogSeeds {
		must(`INSERT INTO foods (id, user_id, name, name_key, basis, kcal, protein, carbs, fat, source, archived, created_at, updated_at)
			VALUES (?, ?, ?, ?, '100g', ?, 1, 2, 3, ?, ?, ?, ?)`, f.id, catalogUsers[f.user], f.name, f.key, f.kcal, f.source,
			f.archived, f.updated, f.updated)
	}
	ana, ivo := catalogSeeds[0], catalogSeeds[1]
	// Ana's older grapes have ten barcodes more, all but the newest ten
	// of the row's left out of a food made of it.
	for _, code := range anasCodes {
		must("INSERT INTO food_barcodes (food_id, user_id, code) VALUES (?, ?, ?)", ana.id, catalogUsers["Ana"], code)
	}
	must(`INSERT INTO food_barcodes (food_id, user_id, code) VALUES (?, ?, '5060000000016')`, ivo.id, catalogUsers["Ivo"])
	must(`INSERT INTO food_servings (food_id, user_id, name, name_key, amount) VALUES (?, ?, 'bobica', 'bobica', 5),
		(?, ?, 'grozd', 'grozd', 150)`, ana.id, catalogUsers["Ana"], ivo.id, catalogUsers["Ivo"])
}

// checkCatalogRows checks the catalog's rows, the grapes' values, id,
// barcodes and servings, and returns the grapes' id.
func checkCatalogRows(t *testing.T, db *DB) string {
	t.Helper()
	var rows, grape, codes, servings string
	var kcal float64
	if err := db.write.QueryRowContext(t.Context(), "SELECT group_concat(name, '|') FROM (SELECT name FROM catalog_foods ORDER BY name)").
		Scan(&rows); err != nil || rows != "BIJELO GROŽĐE|Jabuka" {
		t.Errorf("the catalog's foods = %q, %v; want the grapes, by Ivo's name, and the apple", rows, err)
	}
	if err := db.write.QueryRowContext(t.Context(), "SELECT id, kcal FROM catalog_foods WHERE name_key = 'bijelo grožđe'").
		Scan(&grape, &kcal); err != nil || kcal != 70 {
		t.Fatalf("the grapes' kcal = %v, %v; want Ivo's 70, saved last", kcal, err)
	}
	id, err := uuid.Parse(grape)
	if err != nil || id[6]>>4 != 7 || slices.ContainsFunc(catalogSeeds, func(f catalogSeed) bool { return f.id == grape }) {
		t.Errorf("the grapes' id %q, %v: want a UUIDv7 of its own", grape, err)
	}
	if err := db.write.QueryRowContext(t.Context(), `SELECT
		(SELECT group_concat(code, ' ') FROM (SELECT code FROM catalog_barcodes WHERE catalog_id = ?1 ORDER BY code)),
		(SELECT group_concat(name, ' ') FROM catalog_servings WHERE catalog_id = ?1)`, grape).Scan(&codes, &servings); err != nil {
		t.Fatal(err)
	}
	// The newest ten: Ana's oldest goes.
	if want := strings.Join(append(slices.Clone(anasCodes[1:]), "5060000000016"), " "); codes != want || servings != "grozd" {
		t.Errorf("the grapes' barcodes %q and servings %q; want %q and Ivo's serving", codes, servings, want)
	}
	found, err := db.CatalogFood(t.Context(), id)
	if err != nil || len(found.Barcodes) != 10 || !slices.Contains(found.Barcodes, "5060000000016") || slices.Contains(found.Barcodes, anasCodes[0]) {
		t.Errorf("the grapes' food = %+v, %v; want the newest ten barcodes, Ivo's among them", found, err)
	}
	return grape
}

// checkCatalogLinks checks that both grapes name their row, the apple
// another, and the estimate and the archived food none.
func checkCatalogLinks(t *testing.T, db *DB, grape string) {
	t.Helper()
	var links string
	if err := db.write.QueryRowContext(t.Context(), `SELECT group_concat(id || '=' || ifnull(catalog_id = ?, 'none'), ' ')
		FROM (SELECT id, catalog_id FROM foods ORDER BY id)`, grape).Scan(&links); err != nil {
		t.Fatal(err)
	}
	f := catalogSeeds
	if want := f[0].id + "=1 " + f[1].id + "=1 " + f[2].id + "=0 " + f[3].id + "=none " + f[4].id + "=none"; links != want {
		t.Errorf("the foods' links = %q, want %q", links, want)
	}
}

// checkCatalogSearch checks that a user without the grapes finds them
// however a đ is typed, Ana, who has them, does not, and nobody finds
// the estimate or the archived food.
func checkCatalogSearch(t *testing.T, db *DB, grape string) {
	t.Helper()
	eva := uuid.MustParse(catalogUsers["Eva"])
	for _, q := range []string{"grozde", "grozdje", "grožđe", "bijel"} {
		if found, err := db.FindCatalog(t.Context(), eva, q, 10); err != nil || len(found) != 1 || found[0].ID.String() != grape {
			t.Errorf("Eva's FindCatalog(%q) = %+v, %v; want the grapes", q, found, err)
		}
	}
	if found, err := db.FindCatalog(t.Context(), uuid.MustParse(catalogUsers["Ana"]), "grozd", 10); err != nil || len(found) != 0 {
		t.Errorf("Ana's FindCatalog(grozd) = %+v, %v; want none: she has them", found, err)
	}
	for _, q := range []string{"gulas", "kreker"} {
		if found, err := db.FindCatalog(t.Context(), eva, q, 10); err != nil || len(found) != 0 {
			t.Errorf("Eva's FindCatalog(%q) = %+v, %v; want none", q, found, err)
		}
	}
}

// An undo of a save made before the catalog takes back what the
// migration made of it, a correction since undone or not: a
// correction's values, then a new food's row; and a new food's of
// another's too leaves the row.
func TestUndoOfASaveBeforeTheCatalog(t *testing.T) {
	t.Parallel()

	ivo := uuid.MustParse(catalogUsers["Ivo"])
	apple := uuid.MustParse(catalogSeeds[2].id)
	grapes := uuid.MustParse(catalogSeeds[1].id)
	db := migratedCatalog(t, false, func(db *DB) {
		// The events of Ivo's saves before: the grapes new, the apple new
		// at 50 kcal, and corrected since.
		after := foodAt(t, db, ivo, apple)
		before := after
		before.Nutrients.Kcal = 50
		legacySaves(t, db, ivo, []legacySave{
			{grapes, nil, foodAt(t, db, ivo, grapes)}, {apple, nil, before}, {apple, before, after},
		})
	})
	actor := event.Actor{Kind: event.User, Client: "web"}
	now := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	// A correction since, undone, leaves the row of the save before.
	corrected := foodAt(t, db, ivo, apple)
	corrected.Nutrients.Kcal = 55
	if _, err := db.SaveFood(t.Context(), ivo, actor, corrected, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UndoLast(t.Context(), ivo, actor, now); err != nil {
		t.Fatalf("undo the correction since: %v", err)
	}
	if _, err := db.UndoLast(t.Context(), ivo, actor, now); err != nil {
		t.Fatalf("undo the apple's correction: %v", err)
	}
	if got := findCatalogOf(t, db, "jabuk"); len(got) != 1 || got[0].Nutrients.Kcal != 50 {
		t.Errorf("the catalog's apple after the undo = %+v; want 50 kcal, as before the correction", got)
	}
	// The apple Ivo made, his alone, goes, and its row too.
	if _, err := db.UndoLast(t.Context(), ivo, actor, now.Add(time.Minute)); err != nil {
		t.Fatalf("undo the apple: %v", err)
	}
	if got := findCatalogOf(t, db, "jabuk"); len(got) != 0 {
		t.Errorf("the catalog's apple after Ivo undid it = %+v; want none", got)
	}
	// The grapes Ivo made are Ana's too: their row stays.
	if _, err := db.UndoLast(t.Context(), ivo, actor, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("undo the grapes: %v", err)
	}
	if got := findCatalogOf(t, db, "grozd"); len(got) != 1 {
		t.Errorf("the catalog's grapes after Ivo's undo = %+v; want them, Ana's too", got)
	}
}

// legacySave is a food's save made before the catalog, of the food
// entity, from before to after.
type legacySave struct {
	entity        uuid.UUID
	before, after any
}

// legacySaves writes saves as user's events, oldest first, as the store
// wrote them before the catalog.
func legacySaves(t *testing.T, db *DB, user uuid.UUID, saves []legacySave) {
	t.Helper()
	for i, ev := range saves {
		b, err := stateJSON(ev.before)
		if err != nil {
			t.Fatal(err)
		}
		a, err := stateJSON(ev.after)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.write.ExecContext(t.Context(), `INSERT INTO events (id, user_id, at, actor, client, operation, entity, before_state, after_state)
			VALUES (?, ?, '2026-10-20T08:00:00Z', 'user', 'web', 'food.save', ?, ?, ?)`,
			fmt.Sprintf("0199f0a0-0000-7000-8000-0000000000e%d", i), user.String(), ev.entity.String(), b, a); err != nil {
			t.Fatal(err)
		}
	}
}

// foodAt is user's food id.
func foodAt(t *testing.T, db *DB, user, id uuid.UUID) food.Food {
	t.Helper()
	f, err := db.FoodByID(t.Context(), user, id)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// findCatalogOf is Eva's FindCatalog of query, who has no foods.
func findCatalogOf(t *testing.T, db *DB, query string) []food.Food {
	t.Helper()
	got, err := db.FindCatalog(t.Context(), uuid.MustParse(catalogUsers["Eva"]), query, 10)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// migratedCatalog is a database of catalogSeeds migrated to the
// catalog, the grapes' kcal Ana's and Ivo's alike when same, after
// setup, before the migration.
func migratedCatalog(t *testing.T, same bool, setup func(db *DB)) *DB {
	t.Helper()
	db := openEmpty(t)
	if err := db.migrate(t.Context(), upTo(t, "0029_food_reference.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0029: %v", err)
	}
	seedCatalog(t, db)
	if same {
		if _, err := db.write.ExecContext(t.Context(), "UPDATE foods SET kcal = 69 WHERE id = ?", catalogSeeds[1].id); err != nil {
			t.Fatal(err)
		}
	}
	if setup != nil {
		setup(db)
	}
	if err := db.migrate(t.Context(), upTo(t, "0031_food_catalog_search.sql"), discard()); err != nil {
		t.Fatalf("migrate to 0031: %v", err)
	}
	return db
}

// An undo of a save before the catalog leaves the row of another's food
// saved after it, of the same values, which the migration made the row
// of, even with them gone; and an undo of an estimate's save never
// touches the catalog.
func TestUndoBeforeTheCatalogLeavesOthersRows(t *testing.T) {
	t.Parallel()

	ana := uuid.MustParse(catalogUsers["Ana"])
	grapes := uuid.MustParse(catalogSeeds[0].id)
	db := migratedCatalog(t, true, func(db *DB) {
		after := foodAt(t, db, ana, grapes)
		before := after
		before.Nutrients.Kcal = 60
		legacySaves(t, db, ana, []legacySave{{grapes, before, after}})
	})
	// Ivo, whose grapes the row is of, goes: the row stays his.
	if _, err := db.DeleteUser(t.Context(), uuid.MustParse(catalogUsers["Ivo"])); err != nil {
		t.Fatal(err)
	}
	actor := event.Actor{Kind: event.User, Client: "web"}
	if _, err := db.UndoLast(t.Context(), ana, actor, time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if got := findCatalogOf(t, db, "grozd"); len(got) != 1 || got[0].Nutrients.Kcal != 69 {
		t.Errorf("the grapes after Ana's undo = %+v; want Ivo's 69 kcal, saved after hers", got)
	}

	// Ivo's apple's row names no food now; Eva's estimate of its name
	// and values, undone, leaves it.
	eva := uuid.MustParse(catalogUsers["Eva"])
	estimate := food.Food{
		ID: uuid.NewV7(), Name: "Jabuka", Basis: food.Per100g, Source: food.Estimate,
		Nutrients: food.Nutrients{Kcal: 52, Protein: 1, Carbs: 2, Fat: 3},
	}
	if _, err := db.SaveFood(t.Context(), eva, actor, estimate, time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UndoLast(t.Context(), eva, actor, time.Date(2026, 11, 2, 8, 1, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if got := findCatalogOf(t, db, "jabuk"); len(got) != 1 {
		t.Errorf("the catalog's apple after Eva undid her estimate = %+v; want it", got)
	}
}

// An undo puts back a food's link to its row before as none when the
// row went since, rather than fail.
func TestUndoOfALinkToARowGone(t *testing.T) {
	t.Parallel()

	db := migratedCatalog(t, false, nil)
	ivo := uuid.MustParse(catalogUsers["Ivo"])
	apple := foodAt(t, db, ivo, uuid.MustParse(catalogSeeds[2].id))
	actor := event.Actor{Kind: event.User, Client: "web"}
	now := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	apple.Name = "Jabuka zelena"
	if _, err := db.SaveFood(t.Context(), ivo, actor, apple, now); err != nil {
		t.Fatal(err)
	}
	// The renamed apple took its row along: make the row it had before
	// one that goes, as an undo of another's save may make it go.
	if _, err := db.write.ExecContext(t.Context(), "UPDATE catalog_saves SET link_before = ? WHERE user_id = ?",
		"0199f0a0-0000-7000-8000-0000000000aa", ivo.String()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UndoLast(t.Context(), ivo, actor, now.Add(time.Minute)); err != nil {
		t.Fatalf("undo the rename: %v", err)
	}
	var link sql.NullString
	if err := db.write.QueryRowContext(t.Context(), "SELECT catalog_id FROM foods WHERE id = ?", apple.ID.String()).Scan(&link); err != nil || link.Valid {
		t.Errorf("the apple's link after the undo = %v, %v; want none (its row before went)", link, err)
	}
}

// An undo of the save before the catalog that the migration made a row
// of: a correction gives the row its values before, takes out the
// barcode it brought, but one another's food has too, and leaves
// another's barcodes; a save that made a label's food of an estimate
// takes the row along, and the food out of it.
func TestUndoBeforeTheCatalogOfTheRowsOwnSave(t *testing.T) {
	t.Parallel()

	ivo := uuid.MustParse(catalogUsers["Ivo"])
	grapes := uuid.MustParse(catalogSeeds[1].id)
	apple := uuid.MustParse(catalogSeeds[2].id)
	const shared = "4000000000105"
	db := migratedCatalog(t, false, func(db *DB) {
		if _, err := db.write.ExecContext(t.Context(), "INSERT INTO food_barcodes (food_id, user_id, code) VALUES (?, ?, ?), (?, ?, ?)",
			catalogSeeds[0].id, catalogUsers["Ana"], shared, grapes.String(), ivo.String(), shared); err != nil {
			t.Fatal(err)
		}
		after := foodAt(t, db, ivo, grapes)
		before := after
		before.Nutrients.Kcal, before.Barcodes = 60, nil
		label := foodAt(t, db, ivo, apple)
		estimate := label
		estimate.Source = food.Estimate
		legacySaves(t, db, ivo, []legacySave{{grapes, before, after}, {apple, estimate, label}})
	})
	actor := event.Actor{Kind: event.User, Client: "web"}
	now := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)

	if _, err := db.UndoLast(t.Context(), ivo, actor, now); err != nil {
		t.Fatalf("undo the apple's label: %v", err)
	}
	if got := findCatalogOf(t, db, "jabuk"); len(got) != 0 {
		t.Errorf("the catalog's apple after its label was undone = %+v; want none", got)
	}
	var link sql.NullString
	if err := db.write.QueryRowContext(t.Context(), "SELECT catalog_id FROM foods WHERE id = ?", apple.String()).Scan(&link); err != nil || link.Valid {
		t.Errorf("the apple's link = %v, %v; want none", link, err)
	}

	if _, err := db.UndoLast(t.Context(), ivo, actor, now.Add(time.Minute)); err != nil {
		t.Fatalf("undo the grapes' correction: %v", err)
	}
	got := findCatalogOf(t, db, "grozd")
	if len(got) != 1 || got[0].Nutrients.Kcal != 60 {
		t.Fatalf("the grapes after Ivo's undo = %+v; want 60 kcal", got)
	}
	if !slices.Contains(got[0].Barcodes, shared) || slices.Contains(got[0].Barcodes, "5060000000016") {
		t.Errorf("the grapes' barcodes = %v; want Ana's %s, and not Ivo's own, which his correction brought", got[0].Barcodes, shared)
	}
}
