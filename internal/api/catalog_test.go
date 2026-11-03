package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/food"
	"github.com/zarnautovic/tdeemon/internal/domain/meal"
	"github.com/zarnautovic/tdeemon/internal/domain/user"
)

// catalogEnv is an instance whose other user, Zoe, an admin, has a protein bar of
// a label's, which is in the catalog, and a stew of her estimate, which
// is not; and Ana's session.
type catalogEnv struct {
	mealsEnv
	zoe user.User
	bar food.Food
	// row is the catalog's id of the bar.
	row string
}

// barCode is the bar's barcode.
const barCode = "4006381333931"

func newCatalogEnv(t *testing.T) catalogEnv {
	t.Helper()
	e := catalogEnv{mealsEnv: newMealsEnv(t)}
	e.zoe = e.in.addUser(t, user.RoleAdmin)
	e.bar = food.Food{
		ID: uuid.NewV7(), Name: "Protein bar", Brand: "Brawn", Basis: food.Per100g, Source: food.Label,
		Nutrients: food.Nutrients{Kcal: 380, Protein: 30, Carbs: 40, Fat: 11},
		Servings:  []food.Serving{{Name: "bar", Amount: 55}},
		Aliases:   []string{"Zoe's bar"}, Note: "Zoe's own words",
		Barcodes: []string{barCode},
	}
	stew := food.Food{
		ID: uuid.NewV7(), Name: "Protein stew", Basis: food.Per100g, Source: food.Estimate,
		Nutrients: food.Nutrients{Kcal: 120, Protein: 9, Carbs: 10, Fat: 5},
	}
	for _, f := range []*food.Food{&e.bar, &stew} {
		if _, err := e.in.db.SaveFood(t.Context(), e.zoe.ID, isolationActor, *f, e.now); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := e.in.db.FindCatalog(t.Context(), e.in.user.ID, "protein", 10)
	if err != nil || len(rows) != 1 {
		t.Fatalf("the catalog of protein = %v, %v; want the bar alone", rows, err)
	}
	e.row = rows[0].ID.String()
	return e
}

// find_foods gives, after the user's foods, the catalog's that they have
// not: a label's, without its saver's note and aliases, never an estimate,
// within the limit, and none of the archived.
func TestFindFoodsFindsTheCatalog(t *testing.T) {
	t.Parallel()

	e := newCatalogEnv(t)
	rec := e.in.do(t, http.MethodGet, "/api/v1/foods?query=protein", "", e.session, nil)
	checkJSON(t, rec.Body.Bytes(), fmt.Sprintf(`{"foods": [], "catalog": [
		{"id": %q, "name": "Protein bar", "brand": "Brawn", "basis": "100g", "source": "label",
		 "nutrients": {"kcal": 380, "protein": 30, "carbs": 40, "fat": 11},
		 "servings": [{"name": "bar", "amount": 55}], "barcodes": [%q], "queries": ["protein"]}
	], "unfound": []}`, e.row, barCode))
	// Ana's own protein food first; the limit is of both.
	pudding := food.Food{
		ID: uuid.NewV7(), Name: "Protein pudding", Basis: food.Per100g, Source: food.Label,
		Nutrients: food.Nutrients{Kcal: 90, Protein: 10, Carbs: 8, Fat: 2},
	}
	if _, err := e.in.db.SaveFood(t.Context(), e.in.user.ID, isolationActor, pudding, e.now); err != nil {
		t.Fatal(err)
	}
	for query, want := range map[string]string{
		"protein&limit=1":         `{"foods":[{"id":"` + pudding.ID.String() + `"`,
		"protein&archived=true":   `"catalog":[]`,
		"stew":                    `"catalog":[],"unfound":["stew"]`,
		"brawn&query=protein+bar": `"queries":["brawn","protein bar"]}],"unfound":[]}`,
	} {
		rec := e.in.do(t, http.MethodGet, "/api/v1/foods?query="+query, "", e.session, nil)
		if body := rec.Body.String(); rec.Code != http.StatusOK || !strings.Contains(body, want) {
			t.Errorf("%s: %d %s, want %s", query, rec.Code, body, want)
		}
	}
	rec = e.in.do(t, http.MethodGet, "/api/v1/foods?query=protein&limit=1", "", e.session, nil)
	if strings.Contains(rec.Body.String(), "Protein bar") {
		t.Errorf("limit 1: %s, want Ana's pudding alone", rec.Body.String())
	}
	rec = e.in.do(t, http.MethodGet, "/api/v1/foods?query=protein&limit=2", "", e.session, nil)
	if body := rec.Body.String(); !strings.Contains(body, "Protein pudding") || !strings.Contains(body, `"catalog":[{"id":"`+e.row+`"`) {
		t.Errorf("limit 2: %s, want Ana's pudding, then the bar", body)
	}
}

// A barcode of no food of the user's is the catalog's food of it, and
// Open Food Facts is not asked; with the user's food of its name and
// brand, that too.
func TestLookupBarcodeOfTheCatalog(t *testing.T) {
	t.Parallel()

	e := newCatalogEnv(t)
	asked := withOpenFoodFacts(t, e.in, http.StatusOK, kefir).asked
	rec := e.in.do(t, http.MethodGet, "/api/v1/barcodes/"+barCode, "", e.session, nil)
	if body := rec.Body.String(); rec.Code != http.StatusOK || !strings.Contains(body, `"catalog":{"id":"`+e.row+`"`) ||
		strings.Contains(body, "named_food") || strings.Contains(body, `"food":`) {
		t.Errorf("the bar's code: %d %s, want the catalog's bar", rec.Code, body)
	}
	// Ana's bread has the other code: hers, not the catalog's.
	rec = e.in.do(t, http.MethodGet, "/api/v1/barcodes/3850000000015", "", e.session, nil)
	if body := rec.Body.String(); !strings.Contains(body, `"food":{"id":"`+e.bread+`"`) || strings.Contains(body, "catalog") {
		t.Errorf("the bread's code: %d %s, want Ana's bread", rec.Code, body)
	}
	if asked.Load() != 0 {
		t.Errorf("Open Food Facts was asked %d times", asked.Load())
	}
	// Ana's own bar of its name, without the code.
	mine := food.Food{
		ID: uuid.NewV7(), Name: "Protein bar", Brand: "Brawn", Basis: food.Per100g, Source: food.Estimate,
		Nutrients: food.Nutrients{Kcal: 400, Protein: 25, Carbs: 45, Fat: 13},
	}
	if _, err := e.in.db.SaveFood(t.Context(), e.in.user.ID, isolationActor, mine, e.now); err != nil {
		t.Fatal(err)
	}
	rec = e.in.do(t, http.MethodGet, "/api/v1/barcodes/"+barCode, "", e.session, nil)
	if body := rec.Body.String(); !strings.Contains(body, `"catalog":{"id":"`+e.row+`"`) || !strings.Contains(body, `"named_food":{"id":"`+mine.ID.String()+`"`) {
		t.Errorf("the bar's code with Ana's bar: %d %s, want both", rec.Code, body)
	}
}

// An item of a catalog's food copies it into the user's foods, without
// its saver's words and the barcodes the user's foods have, linked to
// its row: the catalog finds it no more, a retry is the same meal, and
// an undo takes back the meal and the copy, leaving the catalog and
// Zoe's food as they were.
func TestLogMealOfACatalogFood(t *testing.T) {
	t.Parallel()

	e := newCatalogEnv(t)
	// Zoe's bar has the code of Ana's bread too: Ana's bread is no label's,
	// so the catalog's bar has it, and Ana's search leaves the bar out.
	e.bar.Barcodes = []string{barCode, "3850000000015"}
	if _, err := e.in.db.SaveFood(t.Context(), e.zoe.ID, isolationActor, e.bar, e.now); err != nil {
		t.Fatal(err)
	}
	id := meal.NewIDAt(e.now).String()
	body := fmt.Sprintf(`{"id": %q, "items": [{"catalog_id": %q, "servings": 1, "serving": "bar"}]}`, id, e.row)
	rec := e.in.do(t, http.MethodPost, "/api/v1/meals", body, e.session, nil)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"kcal":209`) {
		t.Fatalf("a meal of the catalog's bar: %d %s", rec.Code, rec.Body.String())
	}
	copied := e.copyOfTheBar(t)
	if !strings.Contains(rec.Body.String(), `"food_id":"`+copied.ID.String()+`"`) {
		t.Errorf("the item: %s, want Ana's copy", rec.Body.String())
	}
	if rows, err := e.in.db.FindCatalog(t.Context(), uuid.NewV7(), "protein", 10); err != nil || len(rows) != 1 || rows[0].ID.String() != e.row {
		t.Errorf("the catalog after = %v, %v; want the bar's row, one", rows, err)
	}
	if again := e.in.do(t, http.MethodPost, "/api/v1/meals", body, e.session, nil); again.Code != http.StatusOK {
		t.Errorf("the meal sent again: %d %s, want a retry", again.Code, again.Body.String())
	}
	e.undoesTheCopy(t)
}

// copyOfTheBar is Ana's copy of the bar, which has the bar's values, its
// own code alone, and none of Zoe's words.
func (e catalogEnv) copyOfTheBar(t *testing.T) food.Food {
	t.Helper()
	copied, err := foodNamed(t, e.in.db, e.in.user.ID, "Protein bar", "Brawn")
	if err != nil {
		t.Fatalf("Ana's copy of the bar: %v", err)
	}
	if copied.ID == e.bar.ID || copied.Note != "" || len(copied.Aliases) != 0 || copied.Source != food.Label ||
		len(copied.Servings) != 1 || strings.Join(copied.Barcodes, " ") != barCode {
		t.Errorf("Ana's copy = %+v, want the bar's values, its own code alone, and none of Zoe's words", copied)
	}
	return copied
}

// undoesTheCopy undoes the latest change, the meal of Ana's copy of the
// bar, and checks the copy went with it, and Zoe's bar and its row stay.
func (e catalogEnv) undoesTheCopy(t *testing.T) {
	t.Helper()
	if rec := e.in.do(t, http.MethodPost, "/api/v1/undo", "{}", e.session, nil); rec.Code != http.StatusOK {
		t.Fatalf("the undo: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := foodNamed(t, e.in.db, e.in.user.ID, "Protein bar", "Brawn"); err == nil {
		t.Error("Ana's copy outlived the undo of its meal")
	}
	zoes, err := e.in.db.FoodByID(t.Context(), e.zoe.ID, e.bar.ID)
	if err != nil || zoes.Note != e.bar.Note {
		t.Errorf("Zoe's bar after = %+v, %v; want it as it was", zoes, err)
	}
	row, err := e.in.db.CatalogFood(t.Context(), uuid.MustParse(e.row))
	if err != nil || row.Nutrients.Kcal != 380 || len(row.Barcodes) != 2 {
		t.Errorf("the row after = %+v, %v; want it as it was", row, err)
	}
}

// An item of the catalog is one kind alone, of a row there is; an add
// of one to a meal copies it as a log does.
func TestCatalogItems(t *testing.T) {
	t.Parallel()

	e := newCatalogEnv(t)
	for _, item := range []string{
		fmt.Sprintf(`{"catalog_id": %q, "food_id": %q, "amount": 50, "unit": "g"}`, e.row, e.bread),
		fmt.Sprintf(`{"catalog_id": %q, "name": "Bar", "nutrients": {"kcal": 200, "protein": 1, "carbs": 1, "fat": 1}}`, e.row),
		fmt.Sprintf(`{"catalog_id": %q, "amount": 50, "unit": "g"}`, uuid.NewV7()),
		`{"catalog_id": "nope", "amount": 50, "unit": "g"}`,
	} {
		rec := e.in.do(t, http.MethodPost, "/api/v1/meals", `{"items": [`+item+`]}`, e.session, nil)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: %d %s, want 422", item, rec.Code, rec.Body.String())
		}
	}
	if _, err := foodNamed(t, e.in.db, e.in.user.ID, "Protein bar", "Brawn"); err == nil {
		t.Error("a refused item copied the bar")
	}

	id := meal.NewIDAt(e.now).String()
	e.logged(t, fmt.Sprintf(`{"id": %q, "items": [{"food_id": %q, "amount": 40, "unit": "g"}]}`, id, e.bread))
	add := fmt.Sprintf(`{"ops": [{"op": "add", "item": {"id": %q, "catalog_id": %q, "amount": 50, "unit": "g"}}]}`, uuid.NewV7(), e.row)
	if rec := e.patch(t, id, add); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"kcal":190`) {
		t.Fatalf("an add of the bar: %d %s", rec.Code, rec.Body.String())
	}
	if rec := e.patch(t, id, add); rec.Code != http.StatusOK {
		t.Errorf("the add sent again: %d %s, want a retry", rec.Code, rec.Body.String())
	}
	if got := e.mealOf(t, id); len(got.Items) != 2 {
		t.Errorf("the meal after = %+v, want two items", got.Items)
	}
	if _, err := foodNamed(t, e.in.db, e.in.user.ID, "Protein bar", "Brawn"); err != nil {
		t.Errorf("Ana's copy after the add: %v", err)
	}
	if rows, err := e.in.db.FindCatalog(t.Context(), e.in.user.ID, "protein", 10); err != nil || len(rows) != 0 {
		t.Errorf("Ana's catalog of protein after = %v, %v; want none: the copy is hers", rows, err)
	}
}

// An item of the catalog is told by the row's id and its quantity as
// sent: a retry after the row was renamed, of another basis, is the
// same meal, and Ana's copy keeps what she logged, as the row keeps
// Zoe's latest.
func TestRetryOfACatalogItemAfterTheRowChanged(t *testing.T) {
	t.Parallel()

	e := newCatalogEnv(t)
	id := meal.NewIDAt(e.now).String()
	// A serving's unit said again, as a model may.
	body := fmt.Sprintf(`{"id": %q, "items": [{"catalog_id": %q, "servings": 1, "serving": "bar", "unit": "g"}]}`, id, e.row)
	if rec := e.in.do(t, http.MethodPost, "/api/v1/meals", body, e.session, nil); rec.Code != http.StatusCreated {
		t.Fatalf("a meal of the catalog's bar: %d %s", rec.Code, rec.Body.String())
	}
	e.bar.Name, e.bar.Basis, e.bar.Nutrients.Kcal = "Protein drink", food.Per100ml, 70
	if _, err := e.in.db.SaveFood(t.Context(), e.zoe.ID, isolationActor, e.bar, e.now); err != nil {
		t.Fatal(err)
	}
	if rec := e.in.do(t, http.MethodPost, "/api/v1/meals", body, e.session, nil); rec.Code != http.StatusOK {
		t.Errorf("the meal sent again: %d %s, want a retry", rec.Code, rec.Body.String())
	}
	if copied := e.copyOfTheBar(t); copied.Nutrients.Kcal != 380 {
		t.Errorf("Ana's copy = %+v, want the bar as she logged it", copied)
	}
	row, err := e.in.db.CatalogFood(t.Context(), uuid.MustParse(e.row))
	if err != nil || row.Name != "Protein drink" || row.Nutrients.Kcal != 70 {
		t.Errorf("the row = %+v, %v; want Zoe's latest", row, err)
	}
}

// A catalog's food Ana has a food of the name and brand of is her food:
// the barcode it brings is added to hers, which is linked to the row,
// and the row keeps Zoe's latest values, not Ana's.
func TestACatalogItemOfAFoodTheUserHas(t *testing.T) {
	t.Parallel()

	e := newCatalogEnv(t)
	mine := food.Food{
		ID: uuid.NewV7(), Name: "Protein bar", Brand: "Brawn", Basis: food.Per100g, Source: food.Label,
		Nutrients: food.Nutrients{Kcal: 400, Protein: 25, Carbs: 45, Fat: 13},
	}
	if _, err := e.in.db.SaveFood(t.Context(), e.in.user.ID, isolationActor, mine, e.now); err != nil {
		t.Fatal(err)
	}
	// Zoe's correction after Ana's save: the latest.
	e.bar.Nutrients.Protein = 31
	if _, err := e.in.db.SaveFood(t.Context(), e.zoe.ID, isolationActor, e.bar, e.now); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"items": [{"catalog_id": %q, "amount": 50, "unit": "g"}]}`, e.row)
	rec := e.in.do(t, http.MethodPost, "/api/v1/meals", body, e.session, nil)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"food_id":"`+mine.ID.String()+`"`) ||
		!strings.Contains(rec.Body.String(), `"kcal":200`) {
		t.Fatalf("a meal of the catalog's bar: %d %s, want Ana's bar", rec.Code, rec.Body.String())
	}
	if had, err := e.in.db.FoodByID(t.Context(), e.in.user.ID, mine.ID); err != nil || strings.Join(had.Barcodes, " ") != barCode {
		t.Errorf("Ana's bar after = %+v, %v; want the bar's code", had, err)
	}
	if row, err := e.in.db.CatalogFood(t.Context(), uuid.MustParse(e.row)); err != nil || row.Nutrients.Kcal != 380 {
		t.Errorf("the row after = %+v, %v; want Zoe's 380 kcal", row, err)
	}
}
