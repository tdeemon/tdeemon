package store_test

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/tdeemon/tdeemon/internal/domain/food"
	"github.com/tdeemon/tdeemon/internal/domain/user"
	"github.com/tdeemon/tdeemon/internal/store"
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
	eva := addEva(t, db)
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
	if got := findCatalog(t, db, addEva(t, db), "tuna"); len(got) != 2 {
		t.Errorf("the catalog's tunas after Ana's undo = %+v; want Ivo's and the one Ana renamed, as they are", got)
	}
}

// A barcode stays the product's that had it first: another product's
// save with it leaves it where it is.
func TestABarcodeStaysWithItsFirstProduct(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	saveFood(t, db, ana, tuna(116))
	crackers := bar()
	crackers.Aliases = nil
	crackers = saveFood(t, db, ivo, crackers)
	crackers.Barcodes = []string{"3850104000010"}
	saveFood(t, db, ivo, crackers)
	got, err := db.CatalogFoodByBarcode(t.Context(), "3850104000010")
	if err != nil || got.Name != "Tuna u vlastitom soku" {
		t.Errorf("the barcode's food after Ivo's bar = %+v, %v; want the tuna, which had it first", got, err)
	}
	if bars := findCatalog(t, db, addEva(t, db), "caramel"); len(bars) != 1 || len(bars[0].Barcodes) != 0 {
		t.Errorf("Eva's search for the bar = %+v; want it without the tuna's barcode", bars)
	}
}

// A row keeps food.MaxBarcodes of its barcodes, the newest, so a food
// made of it is valid.
func TestCatalogBarcodes(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
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
	got := findCatalog(t, db, addEva(t, db), "keksi")
	if len(got) != 1 || len(got[0].Barcodes) != food.MaxBarcodes || got[0].Validate() != nil {
		t.Fatalf("Eva's search for keksi = %+v; want one valid food of %d barcodes", got, food.MaxBarcodes)
	}
	if !slices.Contains(got[0].Barcodes, codes(5)[food.MaxBarcodes-1]) || slices.Contains(got[0].Barcodes, codes(0)[0]) {
		t.Errorf("the barcodes kept %v: want Ivo's, the newest", got[0].Barcodes)
	}
}

// An undo gives a row back a barcode the save let go only when no other
// row has it since.
func TestUndoLeavesABarcodeAnotherRowTookSince(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	codes := make([]string, 0, food.MaxBarcodes+1)
	for i := range food.MaxBarcodes + 1 {
		codes = append(codes, ean13(t, 400000000000+i))
	}
	anas := tuna(116)
	anas.Aliases, anas.Barcodes = nil, codes[:food.MaxBarcodes]
	anas = saveFood(t, db, ana, anas)
	ivos := tuna(116)
	ivos.Aliases, ivos.Barcodes = nil, codes[food.MaxBarcodes:]
	saveFood(t, db, ivo, ivos) // the row lets codes[0] go, its oldest
	other := bar()
	other.Aliases, other.Barcodes = nil, codes[:1]
	saveFood(t, db, addEva(t, db), other)
	if _, err := db.UndoLast(t.Context(), ivo, testActor, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got, err := db.CatalogFoodByBarcode(t.Context(), codes[0]); err != nil || got.Name != other.Name {
		t.Errorf("the food of %s after Ivo's undo = %+v, %v; want Eva's bar, which has it since", codes[0], got, err)
	}
	got, err := db.CatalogFoodByBarcode(t.Context(), codes[1])
	if err != nil || !slices.Equal(got.Barcodes, codes[1:food.MaxBarcodes]) {
		t.Errorf("the tuna after Ivo's undo = %+v, %v; want Ana's barcodes but the one Eva's bar has", got, err)
	}
}

// An undo puts a row's barcodes back in their order, so the next save
// lets the oldest go.
func TestUndoKeepsTheBarcodesOrder(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	codes := make([]string, 0, food.MaxBarcodes+1)
	for i := range food.MaxBarcodes + 1 {
		codes = append(codes, ean13(t, 400000000000+i))
	}
	anas := tuna(116)
	anas.Aliases, anas.Barcodes = nil, codes[:food.MaxBarcodes]
	saveFood(t, db, ana, anas)
	ivos := tuna(116)
	ivos.Aliases, ivos.Barcodes = nil, codes[food.MaxBarcodes:food.MaxBarcodes+1]
	saveFood(t, db, ivo, ivos)
	if _, err := db.UndoLast(t.Context(), ivo, testActor, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	saveFood(t, db, ivo, ivos)
	got, err := db.CatalogFoodByBarcode(t.Context(), codes[1])
	if want := codes[1 : food.MaxBarcodes+1]; err != nil || !slices.Equal(got.Barcodes, want) {
		t.Errorf("the tuna's barcodes after Ivo's undo and save again = %v, %v; want %v, the oldest gone", got.Barcodes, err, want)
	}
}

// Two saves undone newest first, as a meal's new foods are, put the
// catalog back whole.
func TestUndoOfTwoSavesPutsBothBack(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	ivos := tuna(116)
	ivos.Aliases = nil
	saveFood(t, db, ivo, ivos)
	ivosBar := bar()
	ivosBar.Aliases = nil
	saveFood(t, db, ivo, ivosBar)
	anas := tuna(120)
	anas.Aliases, anas.Barcodes = nil, nil
	saveFood(t, db, ana, anas)
	// Ana's bar goes into Ivo's bar's row, by its name, and leaves the
	// tuna its barcode.
	other := bar()
	other.Aliases, other.Barcodes = nil, []string{"3850104000010"}
	saveFood(t, db, ana, other)
	for i := range 2 {
		if _, err := db.UndoLast(t.Context(), ana, testActor, t0.Add(time.Duration(i+1)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := db.CatalogFoodByBarcode(t.Context(), "3850104000010")
	if err != nil || got.Name != "Tuna u vlastitom soku" || got.Nutrients.Kcal != 116 {
		t.Errorf("the tuna after Ana undid both = %+v, %v; want Ivo's, at 116 kcal, with its barcode", got, err)
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

// addEva adds a third user, Eva, to db and returns her id.
func addEva(t *testing.T, db *store.DB) uuid.UUID {
	t.Helper()
	u := newUser("eva@example.com")
	if err := db.CreateUser(t.Context(), u, ""); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return u.ID
}

// A copy of the catalog's food (CopyOf) is linked to that row and leaves
// it as it is, though its values are older than the row's or another
// save renamed it: the catalog keeps the latest save of a food, and a
// copy is none. Its undo takes the copy and its link back alone.
func TestACopyLeavesItsRow(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	saveFood(t, db, ana, tuna(116))
	row, err := db.CatalogFoodByBarcode(t.Context(), "3850104000010")
	if err != nil {
		t.Fatal(err)
	}
	// Read before Ana renamed and corrected her tuna, so its row.
	stale := row
	stale.ID, stale.CopyOf, stale.Barcodes = uuid.NewV7(), row.ID, nil
	anas, err := db.FoodByKey(t.Context(), ana, row.Name, row.Brand)
	if err != nil {
		t.Fatal(err)
	}
	anas.Name, anas.Nutrients.Kcal = "Tuna", 120
	saveFood(t, db, ana, anas)

	saveFood(t, db, ivo, stale)
	got, err := db.CatalogFood(t.Context(), row.ID)
	if err != nil || got.Name != "Tuna" || got.Nutrients.Kcal != 120 || catalogRows(t, db) != 1 {
		t.Errorf("the row after Ivo's copy = %+v, %v (%d rows); want Ana's latest, alone", got, err, catalogRows(t, db))
	}
	if found := findCatalog(t, db, ivo, "tuna"); len(found) != 0 {
		t.Errorf("Ivo's catalog of tuna = %v, want none: his copy names the row", found)
	}

	// The row changed under its version, as an undo of a save before the
	// catalog changes it: the copy's undo leaves it so.
	if err := db.WriteTx(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(t.Context(), "UPDATE catalog_foods SET kcal = 99 WHERE id = ?", row.ID.String())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.UndoLast(t.Context(), ivo, testActor, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got, err := db.CatalogFood(t.Context(), row.ID); err != nil || got.Name != "Tuna" || got.Nutrients.Kcal != 99 {
		t.Errorf("the row after the undo of Ivo's copy = %+v, %v; want it as it was", got, err)
	}
	if found := findCatalog(t, db, ivo, "tuna"); len(found) != 1 {
		t.Errorf("Ivo's catalog of tuna after the undo = %v, want the row", found)
	}
}

// A copy of a row no longer there goes into the catalog as any save.
func TestACopyOfARowGone(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	gone := tuna(116)
	gone.Name, gone.CopyOf = "Sardines", uuid.NewV7()
	saveFood(t, db, ivo, gone)
	if found := findCatalog(t, db, ana, "sardines"); len(found) != 1 || found[0].ID == gone.CopyOf {
		t.Errorf("Ana's catalog of sardines = %v, want Ivo's copy as a new row", found)
	}
}

// A row whose foods are all of no declared values now (a plate saved as a
// label's, then as an estimate) is no longer shown: not in a search, by
// its id or its barcode; an undo of the estimate shows it again.
func TestAnEstimateHidesItsRow(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	plate := saveFood(t, db, ana, tuna(116))
	row, err := db.CatalogFoodByBarcode(t.Context(), "3850104000010")
	if err != nil {
		t.Fatal(err)
	}
	plate.Source = food.Estimate
	saveFood(t, db, ana, plate)
	shown := func() (bool, bool, bool) {
		t.Helper()
		_, byID := db.CatalogFood(t.Context(), row.ID)
		_, byCode := db.CatalogFoodByBarcode(t.Context(), "3850104000010")
		return len(findCatalog(t, db, ivo, "tuna")) > 0, byID == nil, byCode == nil
	}
	if found, byID, byCode := shown(); found || byID || byCode {
		t.Errorf("the plate's row after the estimate: found %v, by id %v, by code %v; want none", found, byID, byCode)
	}
	if _, err := db.UndoLast(t.Context(), ana, testActor, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if found, byID, byCode := shown(); !found || !byID || !byCode {
		t.Errorf("the row after the undo: found %v, by id %v, by code %v; want it back", found, byID, byCode)
	}
}

// A row a food of declared values names still, another user's or an
// archived one, stays shown when another of its foods becomes an
// estimate.
func TestAnEstimateLeavesASharedRow(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	eva := addEva(t, db)
	anas := saveFood(t, db, ana, tuna(116))
	ivos := tuna(116)
	ivos.Barcodes = nil
	ivos = saveFood(t, db, ivo, ivos)
	anas.Source = food.Estimate
	saveFood(t, db, ana, anas)
	if found := findCatalog(t, db, eva, "tuna"); len(found) != 1 {
		t.Errorf("Eva's catalog of tuna with Ivo's label = %v, want the row", found)
	}
	if err := db.SetFoodArchived(t.Context(), ivo, testActor, ivos.ID, true, t0); err != nil {
		t.Fatal(err)
	}
	if found := findCatalog(t, db, eva, "tuna"); len(found) != 1 {
		t.Errorf("Eva's catalog of tuna with Ivo's archived = %v, want the row", found)
	}
}

// makeUser makes userID a user of no admin's role.
func makeUser(t *testing.T, db *store.DB, userID uuid.UUID) {
	t.Helper()
	role := user.RoleUser
	if _, err := db.ChangeUser(t.Context(), userID, store.UserChange{Role: &role}, t0); err != nil {
		t.Fatalf("ChangeUser: %v", err)
	}
}

// Only an admin's save writes the catalog: another user's declared food,
// a product the catalog has or a new one, stays theirs, and leaves the
// row as the admin's save left it. Their copy of a row is linked to it,
// and made an estimate it hides the row from no one.
func TestCatalogTakesAnAdminsFoodsAlone(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	eva := addEva(t, db)
	makeUser(t, db, ivo)
	saveFood(t, db, ana, tuna(116))
	row, err := db.CatalogFoodByBarcode(t.Context(), "3850104000010")
	if err != nil {
		t.Fatal(err)
	}

	ivos := tuna(300)
	ivos.Barcodes = []string{"3850104000027"}
	saveFood(t, db, ivo, ivos)
	banana := tuna(89)
	banana.Name, banana.Brand, banana.Source, banana.Aliases = "Banana", "", food.Reference, nil
	banana.Barcodes = []string{"2000000000015"}
	saveFood(t, db, ivo, banana)
	if got, err := db.CatalogFood(t.Context(), row.ID); err != nil || got.Nutrients.Kcal != 116 || slices.Contains(got.Barcodes, "3850104000027") {
		t.Errorf("the row after Ivo's tuna = %+v, %v; want Ana's", got, err)
	}
	if n := catalogRows(t, db); n != 1 {
		t.Errorf("the catalog after Ivo's banana has %d rows, want Ana's tuna alone", n)
	}
	if found := findCatalog(t, db, eva, "banana"); len(found) != 0 {
		t.Errorf("Eva's catalog of banana = %v, want none", found)
	}
	if _, err := db.CatalogFoodByBarcode(t.Context(), "2000000000015"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("the catalog's food of Ivo's barcode: %v, want ErrNotFound", err)
	}

	copied := row
	copied.ID, copied.CopyOf, copied.Barcodes = uuid.NewV7(), row.ID, nil
	copied = saveFood(t, db, ivo, copied)
	if _, err := db.DeleteUser(t.Context(), ana); err != nil {
		t.Fatal(err)
	}
	copied.Source = food.Estimate
	saveFood(t, db, ivo, copied)
	if found := findCatalog(t, db, eva, "tuna"); len(found) != 1 || found[0].ID != row.ID {
		t.Errorf("Eva's catalog of tuna after Ivo's copy became an estimate = %v, want Ana's row", found)
	}
}

// A household member who is no admin, saving the household's food of a
// row as an estimate, hides the row from no one; nor does an admin's
// estimate come back into view when they are an admin no longer.
func TestNoAdminHidesARow(t *testing.T) {
	t.Parallel()

	db, ana, ivo, eva := withHousehold(t)
	makeUser(t, db, ivo)
	saveFood(t, db, ana, tuna(116))
	ivos := copyNamed(t, db, ivo, tuna(116).Name)
	ivos.Source, ivos.Nutrients.Kcal = food.Estimate, 500
	saveFood(t, db, ivo, ivos)
	if found := findCatalog(t, db, eva, "tuna"); len(found) != 1 || found[0].Nutrients.Kcal != 116 {
		t.Errorf("Eva's catalog of tuna after Ivo's estimate = %v, want Ana's row", found)
	}

	// Ana saves the household's estimate as an estimate of hers: no food
	// of the row is declared now, so her save hides it.
	estimate := copyNamed(t, db, ana, tuna(116).Name)
	estimate.Nutrients.Kcal = 510
	saveFood(t, db, ana, estimate)
	if found := findCatalog(t, db, eva, "tuna"); len(found) != 0 {
		t.Errorf("Eva's catalog of tuna after Ana's estimate = %v, want none", found)
	}
	// Ana renames her tuna, the household's estimate: still one row,
	// which her declared save shows anew.
	anas := copyNamed(t, db, ana, tuna(116).Name)
	anas.Name, anas.Source, anas.Nutrients.Kcal, anas.Barcodes = "Tuna", food.Label, 120, nil
	saveFood(t, db, ana, anas)
	if found := findCatalog(t, db, eva, "tuna"); len(found) != 1 || found[0].Name != "Tuna" || catalogRows(t, db) != 1 {
		t.Errorf("Eva's catalog of tuna after Ana's rename = %v (%d rows), want her row renamed", found, catalogRows(t, db))
	}

	plate := tuna(116)
	plate.Name, plate.Barcodes, plate.Aliases = "Plate", nil, nil
	plate = saveFood(t, db, eva, plate)
	plate.Source = food.Estimate
	saveFood(t, db, eva, plate)
	makeUser(t, db, eva)
	if found := findCatalog(t, db, ivo, "plate"); len(found) != 0 {
		t.Errorf("Ivo's catalog of the plate Eva made an estimate, then no admin = %v, want none", found)
	}
}
