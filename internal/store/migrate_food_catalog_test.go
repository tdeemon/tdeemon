package store

import (
	"slices"
	"testing"
	"uuid"
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
	must(`INSERT INTO food_barcodes (food_id, user_id, code) VALUES (?, ?, '3850104000010'), (?, ?, '5060000000016')`,
		ana.id, catalogUsers["Ana"], ivo.id, catalogUsers["Ivo"])
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
	if codes != "3850104000010 5060000000016" || servings != "grozd" {
		t.Errorf("the grapes' barcodes %q and servings %q; want both barcodes and Ivo's serving", codes, servings)
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
