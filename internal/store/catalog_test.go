package store_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/food"
	"github.com/zarnautovic/tdeemon/internal/store"
)

// tuna is a label's food, with a serving, an alias, a note and a
// barcode; its kcal as given.
func tuna(kcal float64) food.Food {
	return food.Food{
		ID: uuid.NewV7(), Name: "Tuna u vlastitom soku", Brand: "Eva", Basis: food.Per100g, Source: food.Label,
		Nutrients: food.Nutrients{Kcal: kcal, Protein: 26, Carbs: 0, Fat: 1},
		Servings:  []food.Serving{{Name: "can", Amount: 80}},
		Aliases:   []string{"Anina tuna"},
		Note:      "Ana's own words",
		Barcodes:  []string{"3850104000010"},
	}
}

// findCatalog is FindCatalog's answer for userID, failing on an error.
func findCatalog(t *testing.T, db *store.DB, userID uuid.UUID, query string) []food.Food {
	t.Helper()
	got, err := db.FindCatalog(t.Context(), userID, query, 10)
	if err != nil {
		t.Fatalf("FindCatalog(%q): %v", query, err)
	}
	return got
}

// catalogRows counts the catalog's foods.
func catalogRows(t *testing.T, db *store.DB) int {
	t.Helper()
	var n int
	if err := db.Read().QueryRowContext(t.Context(), "SELECT count(*) FROM catalog_foods").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// catalogText is every text of the catalog's tables, the search's
// included.
func catalogText(t *testing.T, db *store.DB) string {
	t.Helper()
	var s string
	if err := db.Read().QueryRowContext(t.Context(), `SELECT group_concat(v, char(10)) FROM (
		SELECT id || name || brand || source AS v FROM catalog_foods
		UNION ALL SELECT name FROM catalog_servings
		UNION ALL SELECT code FROM catalog_barcodes
		UNION ALL SELECT name || brand FROM catalog_fts)`).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// The catalog takes a food of declared values in use, a label's, Open
// Food Facts' or a food table's, without the user's note or aliases; no
// estimate, no user's word, no recipe, no archived food; and its ID is
// no food's.
func TestCatalogTakesDeclaredFoodsAlone(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	anas := saveFood(t, db, ana, tuna(116))
	for _, f := range []food.Food{
		{Name: "Ana's canteen stew", Source: food.Estimate},
		{Name: "Ana's granola", Source: food.User},
		{Name: "Ana's pot of soup", Source: food.Dish},
		{Name: "Ana's old crackers", Source: food.Label, Archived: true},
	} {
		f.ID, f.Basis = uuid.NewV7(), food.Per100g
		f.Nutrients = food.Nutrients{Kcal: 100, Protein: 1, Carbs: 1, Fat: 1}
		saveFood(t, db, ana, f)
	}
	egg := food.Food{
		ID: uuid.NewV7(), Name: "Jaje", Basis: food.PerPortion, PortionGrams: 50, Source: food.Reference,
		Nutrients: food.Nutrients{Kcal: 72, Protein: 6.3, Carbs: 0.4, Fat: 4.8},
	}
	saveFood(t, db, ana, egg)
	bar := withBarcode(bar(), "5060000000016")
	bar.Source = food.OpenFoodFacts
	saveFood(t, db, ivo, bar)

	if n := catalogRows(t, db); n != 3 {
		t.Errorf("the catalog has %d foods, want 3: the tuna, the egg, the bar", n)
	}
	all := catalogText(t, db)
	for _, s := range []string{"stew", "granola", "soup", "crackers", "Anina", "own words"} {
		if strings.Contains(all, s) {
			t.Errorf("the catalog holds %q: %s", s, all)
		}
	}
	got := findCatalog(t, db, ivo, "tuna")
	if len(got) != 1 {
		t.Fatalf("Ivo's search for tuna = %+v, want Ana's tuna", got)
	}
	want := tuna(116)
	want.ID, want.Aliases, want.Note = got[0].ID, nil, ""
	if !equalFoods(got[0], want) {
		t.Errorf("the catalog's tuna = %+v, want %+v", got[0], want)
	}
	if got[0].ID[6]>>4 != 7 || got[0].ID == anas.ID {
		t.Errorf("the catalog's id %v: want a UUIDv7 of its own", got[0].ID)
	}
	if eggs := findCatalog(t, db, ivo, "jaj"); len(eggs) != 1 || eggs[0].PortionGrams != 50 || eggs[0].Source != food.Reference {
		t.Errorf("Ivo's search for jaj = %+v, want the egg, a food table's, of 50 g", eggs)
	}
}

// equalFoods compares two foods' fields that the catalog keeps.
func equalFoods(a, b food.Food) bool {
	return a.ID == b.ID && a.Name == b.Name && a.Brand == b.Brand && a.Basis == b.Basis && a.Source == b.Source &&
		a.Nutrients.Kcal == b.Nutrients.Kcal && a.Nutrients.Protein == b.Nutrients.Protein &&
		a.Nutrients.Carbs == b.Nutrients.Carbs && a.Nutrients.Fat == b.Nutrients.Fat &&
		slices.Equal(a.Servings, b.Servings) && slices.Equal(a.Barcodes, b.Barcodes) &&
		slices.Equal(a.Aliases, b.Aliases) && a.Note == b.Note && a.PortionGrams == b.PortionGrams
}

// A product is one row, by its name and brand or a barcode, whose values
// are the latest save's of any user's; a food renamed takes its row
// along.
func TestCatalogKeepsTheLatestSave(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	anas := saveFood(t, db, ana, tuna(116))
	ivos := tuna(120)
	ivos.Name, ivos.Barcodes, ivos.Aliases = "TUNA u vlastitom soku", nil, nil
	saveFood(t, db, ivo, ivos)
	if n := catalogRows(t, db); n != 1 {
		t.Fatalf("the catalog has %d foods after two of one name, want 1", n)
	}
	row, err := db.CatalogFoodByBarcode(t.Context(), "3850104000010")
	if err != nil || row.Nutrients.Kcal != 120 || row.Name != "TUNA u vlastitom soku" {
		t.Errorf("the tuna after Ivo's save = %+v, %v; want Ivo's 120 kcal and name", row, err)
	}

	anas.Nutrients.Kcal, anas.Name = 118, "Tuna u vlastitom soku, 160 g"
	anas.Servings = []food.Serving{{Name: "can", Amount: 120}}
	saveFood(t, db, ana, anas)
	got, err := db.CatalogFood(t.Context(), row.ID)
	if err != nil || got.Nutrients.Kcal != 118 || got.Name != anas.Name || len(got.Servings) != 1 || got.Servings[0].Amount != 120 {
		t.Errorf("the tuna after Ana's correction = %+v, %v; want her 118 kcal, new name and can of 120 g", got, err)
	}
	if n := catalogRows(t, db); n != 1 {
		t.Errorf("the catalog has %d foods after Ana renamed hers, want 1", n)
	}

	// Another name, the barcode: the same product, by its last name.
	tonno := tuna(117)
	tonno.Name, tonno.Brand, tonno.Aliases = "Tonno al naturale", "Eva", nil
	saveFood(t, db, ivo, tonno)
	got, err = db.CatalogFood(t.Context(), row.ID)
	if err != nil || got.Name != "Tonno al naturale" || got.Nutrients.Kcal != 117 {
		t.Errorf("the tuna after Ivo's save by its barcode = %+v, %v; want Tonno al naturale, 117 kcal", got, err)
	}
	if n := catalogRows(t, db); n != 1 {
		t.Errorf("the catalog has %d foods after a save by the barcode, want 1", n)
	}
}

// A user's search of the catalog leaves out what they have already: a
// food of theirs that names the row, or has its name and brand, or a
// barcode of it, archived too.
func TestFindCatalogLeavesOutTheUsersOwn(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	saveFood(t, db, ana, tuna(116))
	if got := findCatalog(t, db, ana, "tuna"); len(got) != 0 {
		t.Errorf("Ana's own search = %+v, want none: the tuna is hers", got)
	}
	if got := findCatalog(t, db, ivo, "tun vlast"); len(got) != 1 {
		t.Fatalf("Ivo's search = %+v, want the tuna", got)
	}

	other := bar()
	other.Name, other.Brand, other.Barcodes, other.Source = "My tin", "", []string{"3850104000010"}, food.User
	saveFood(t, db, ivo, other)
	if got := findCatalog(t, db, ivo, "tuna"); len(got) != 0 {
		t.Errorf("Ivo's search with a food of the barcode = %+v, want none", got)
	}

	db2, ana2, ivo2 := withTwoUsers(t)
	saveFood(t, db2, ana2, tuna(116))
	mine := tuna(100)
	mine.Barcodes, mine.Source = nil, food.Estimate
	saved := saveFood(t, db2, ivo2, mine)
	if err := db2.SetFoodArchived(t.Context(), ivo2, testActor, saved.ID, true, t0); err != nil {
		t.Fatal(err)
	}
	if got := findCatalog(t, db2, ivo2, "tuna"); len(got) != 0 {
		t.Errorf("Ivo's search with an archived food of the name = %+v, want none", got)
	}

	// The name's matches first, then the shorter name.
	db3, ana3, ivo3 := withTwoUsers(t)
	for _, n := range []string{"Sir gauda", "Gauda", "Pizza", "Pizza sa sirom gauda"} {
		f := tuna(100)
		f.Name, f.Brand, f.Barcodes, f.Aliases = n, "", nil, nil
		saveFood(t, db3, ana3, f)
	}
	var names []string
	for _, f := range findCatalog(t, db3, ivo3, "gaud") {
		names = append(names, f.Name)
	}
	if want := []string{"Gauda", "Sir gauda", "Pizza sa sirom gauda"}; !slices.Equal(names, want) {
		t.Errorf("Ivo's search for gaud = %q, want %q", names, want)
	}
	if got := findCatalog(t, db3, ivo3, "grozd"); len(got) != 0 {
		t.Errorf("a search of no match = %+v", got)
	}
}

// Undo puts the catalog back: a food undone takes along a row no one
// else's food names, and a correction undone gives the row its values
// before.
func TestUndoPutsTheCatalogBack(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	saveFood(t, db, ana, tuna(116))
	if _, err := db.UndoLast(t.Context(), ana, testActor, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if n := catalogRows(t, db); n != 0 {
		t.Errorf("the catalog has %d foods after Ana undid her only one, want none", n)
	}

	anas := saveFood(t, db, ana, tuna(116))
	ivos := tuna(116)
	ivos.Barcodes = nil
	saveFood(t, db, ivo, ivos)
	if _, err := db.UndoLast(t.Context(), ivo, testActor, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if n := catalogRows(t, db); n != 1 {
		t.Errorf("the catalog has %d foods after Ivo undid his tuna, want Ana's", n)
	}

	anas.Nutrients.Kcal = 300
	saveFood(t, db, ana, anas)
	if _, err := db.UndoLast(t.Context(), ana, testActor, t0.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, err := db.CatalogFoodByBarcode(t.Context(), "3850104000010")
	if err != nil || got.Nutrients.Kcal != 116 {
		t.Errorf("the tuna after Ana undid her correction = %+v, %v; want 116 kcal", got, err)
	}
}

// An undo puts back the row as the save found it, another user's values
// in it, and a row the save did not make stays, even with no food left
// that names it; a save that came since stays as it is.
func TestUndoPutsBackAnotherUsersRow(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	saveFood(t, db, ana, tuna(116))
	ivos := tuna(120)
	ivos.Aliases = nil
	ivos.Barcodes = []string{"5060000000016"}
	saveFood(t, db, ivo, ivos)
	if _, err := db.UndoLast(t.Context(), ivo, testActor, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, err := db.CatalogFoodByBarcode(t.Context(), "3850104000010")
	if err != nil || got.Nutrients.Kcal != 116 || !slices.Equal(got.Barcodes, []string{"3850104000010"}) {
		t.Errorf("the tuna after Ivo undid his save = %+v, %v; want Ana's 116 kcal and her barcode alone", got, err)
	}

	if _, err := db.DeleteUser(t.Context(), ana); err != nil {
		t.Fatal(err)
	}
	saveFood(t, db, ivo, ivos)
	if _, err := db.UndoLast(t.Context(), ivo, testActor, t0.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got, err := db.CatalogFood(t.Context(), got.ID); err != nil || got.Nutrients.Kcal != 116 {
		t.Errorf("Ana's tuna after Ivo undid his save, she gone = %+v, %v; want it, at 116 kcal", got, err)
	}

	// Eva's save since stays when Ivo undoes his.
	eva := addUser(t, db, "eva@example.com")
	saveFood(t, db, ivo, ivos)
	evas := tuna(118)
	evas.Aliases, evas.Barcodes = nil, nil
	saveFood(t, db, eva, evas)
	if _, err := db.UndoLast(t.Context(), ivo, testActor, t0.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got, err := db.CatalogFood(t.Context(), got.ID); err != nil || got.Nutrients.Kcal != 118 {
		t.Errorf("the tuna after Ivo undid his save before Eva's = %+v, %v; want Eva's 118 kcal", got, err)
	}
}

// An undo leaves the row when another took its name since, and still
// puts the user's food back.
func TestUndoLeavesARowWhoseNameWasTaken(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	anas := tuna(116)
	anas.Barcodes = nil
	anas = saveFood(t, db, ana, anas)
	renamed := anas
	renamed.Name = "Tuna 160 g"
	saveFood(t, db, ana, renamed)
	ivos := tuna(120)
	ivos.Aliases, ivos.Barcodes = nil, nil
	saveFood(t, db, ivo, ivos)
	if undone, err := db.UndoLast(t.Context(), ana, testActor, t0.Add(time.Minute)); err != nil {
		t.Fatalf("Ana's undo of her rename = %+v, %v; want it done, not passed over", undone, err)
	}
	if got, err := db.FoodByID(t.Context(), ana, anas.ID); err != nil || got.Name != anas.Name {
		t.Errorf("Ana's food after her undo = %+v, %v; want its name back", got, err)
	}
	if got := findCatalog(t, db, addUser(t, db, "eva@example.com"), "tuna"); len(got) != 2 {
		t.Errorf("the catalog's tunas after Ana's undo = %+v; want Ivo's and the one Ana renamed, as they are", got)
	}
}

// An undo leaves the row when a later save took a barcode of it.
func TestUndoLeavesARowWhoseBarcodeWasTaken(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	anas := saveFood(t, db, ana, tuna(116))
	anas.Nutrients.Kcal = 118
	saveFood(t, db, ana, anas)
	crackers := bar()
	crackers.Aliases = nil
	crackers = saveFood(t, db, ivo, crackers)
	crackers.Barcodes = []string{"3850104000010"}
	saveFood(t, db, ivo, crackers)
	if _, err := db.UndoLast(t.Context(), ana, testActor, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, err := db.CatalogFoodByBarcode(t.Context(), "3850104000010")
	if err != nil || got.Name != crackers.Name {
		t.Errorf("the barcode's food after Ana's undo = %+v, %v; want Ivo's bar, saved since", got, err)
	}
	if tunas := findCatalog(t, db, ivo, "tuna"); len(tunas) != 1 || tunas[0].Nutrients.Kcal != 118 {
		t.Errorf("the tuna after Ana's undo = %+v; want her 118 kcal, left as it is", tunas)
	}
}

// A barcode taken from another row goes back to it with an undo, and a
// row keeps food.MaxBarcodes of its barcodes, the newest, so a food made
// of it is valid.
func TestCatalogBarcodes(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	saveFood(t, db, ana, tuna(116))
	other := bar()
	other.Aliases, other.Barcodes = nil, []string{"3850104000010"}
	saveFood(t, db, ivo, other)
	moved, err := db.CatalogFoodByBarcode(t.Context(), "3850104000010")
	if err != nil || moved.Name != other.Name {
		t.Fatalf("the barcode's food after Ivo's save = %+v, %v; want his bar", moved, err)
	}
	if _, err := db.UndoLast(t.Context(), ivo, testActor, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if back, err := db.CatalogFoodByBarcode(t.Context(), "3850104000010"); err != nil || back.Name != "Tuna u vlastitom soku" {
		t.Errorf("the barcode's food after Ivo's undo = %+v, %v; want the tuna again", back, err)
	}

	codes := func(first int) []string {
		var out []string
		for i := first; i < first+food.MaxBarcodes; i++ {
			out = append(out, ean13(t, 400000000000+i))
		}
		return out
	}
	anas, ivos := tuna(116), tuna(116)
	anas.Name, anas.Aliases, anas.Barcodes = "Keksi", nil, codes(0)
	ivos.Name, ivos.Aliases, ivos.Barcodes = "Keksi", nil, codes(5)
	saveFood(t, db, ana, anas)
	saveFood(t, db, ivo, ivos)
	got := findCatalog(t, db, addUser(t, db, "eva@example.com"), "keksi")
	if len(got) != 1 || len(got[0].Barcodes) != food.MaxBarcodes || got[0].Validate() != nil {
		t.Fatalf("Eva's search for keksi = %+v; want one valid food of %d barcodes", got, food.MaxBarcodes)
	}
	if !slices.Contains(got[0].Barcodes, codes(5)[food.MaxBarcodes-1]) || slices.Contains(got[0].Barcodes, codes(0)[0]) {
		t.Errorf("the barcodes kept %v: want Ivo's, the newest", got[0].Barcodes)
	}
}

// ean13 is the EAN-13 of the 12 digits of n and its check digit.
func ean13(t *testing.T, n int) string {
	t.Helper()
	s := fmt.Sprintf("%012d", n)
	sum := 0
	for i, c := range s {
		d := int(c - '0')
		if i%2 == 1 {
			d *= 3
		}
		sum += d
	}
	return s + fmt.Sprint((10-sum%10)%10)
}

// A user's deletion leaves the catalog: what they added is the
// instance's.
func TestCatalogOutlivesItsUser(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	saveFood(t, db, ana, tuna(116))
	if _, err := db.DeleteUser(t.Context(), ana); err != nil {
		t.Fatal(err)
	}
	if got := findCatalog(t, db, ivo, "tuna"); len(got) != 1 {
		t.Errorf("Ivo's search after Ana's deletion = %+v, want the tuna", got)
	}
	if _, err := db.CatalogFood(t.Context(), uuid.NewV7()); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("CatalogFood of no row: %v, want ErrNotFound", err)
	}
	if _, err := db.CatalogFoodByBarcode(t.Context(), "4000000000006"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("CatalogFoodByBarcode of no row: %v, want ErrNotFound", err)
	}
}

// addUser adds a user of email to db and returns their id.
func addUser(t *testing.T, db *store.DB, email string) uuid.UUID {
	t.Helper()
	u := newUser(email)
	if err := db.CreateUser(t.Context(), u, ""); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return u.ID
}
