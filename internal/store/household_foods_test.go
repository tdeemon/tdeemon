package store_test

import (
	"database/sql"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/food"
	"github.com/zarnautovic/tdeemon/internal/domain/meal"
	"github.com/zarnautovic/tdeemon/internal/domain/recipe"
	"github.com/zarnautovic/tdeemon/internal/store"
)

// inHousehold puts users into one household, as an accepted invite will
// (story 408): the rows alone.
func inHousehold(t *testing.T, db *store.DB, users ...uuid.UUID) {
	t.Helper()
	h := uuid.NewV7().String()
	exec(t, db, "INSERT INTO households (id, created_at) VALUES (?, '2026-11-02T08:00:00Z')", h)
	for _, u := range users {
		exec(t, db, "INSERT INTO household_members (user_id, household_id, joined_at) VALUES (?, ?, '2026-11-02T08:00:00Z')", u.String(), h)
	}
}

// withHousehold is Ana and Ivo in a household, and Eva outside it.
func withHousehold(t *testing.T) (db *store.DB, ana, ivo, eva uuid.UUID) {
	t.Helper()
	db, ana, ivo = withTwoUsers(t)
	eva = addEva(t, db)
	inHousehold(t, db, ana, ivo)
	return db, ana, ivo, eva
}

// sameShared reports whether a and b are one food's copies: every field
// but the id, the note, the aliases and archived.
func sameShared(a, b food.Food) bool {
	a.ID, a.Note, a.Aliases, a.Archived = b.ID, b.Note, b.Aliases, b.Archived
	return reflect.DeepEqual(a, b)
}

// copyNamed is userID's food of name, of any brand, archived too,
// failing the test without one.
func copyNamed(t *testing.T, db *store.DB, userID uuid.UUID, name string) food.Food {
	t.Helper()
	f, ok := named(t, db, userID, name)
	if !ok {
		t.Fatalf("%v has no %s", userID, name)
	}
	return f
}

// named is userID's food of name, of any brand, archived too.
func named(t *testing.T, db *store.DB, userID uuid.UUID, name string) (food.Food, bool) {
	t.Helper()
	for _, archived := range []bool{false, true} {
		fs, _, err := db.ListFoods(t.Context(), userID, archived, "", store.ByName, store.MaxFoodList, 0)
		if err != nil {
			t.Fatal(err)
		}
		for i := range fs {
			if fs[i].Name == name {
				return fs[i], true
			}
		}
	}
	return food.Food{}, false
}

// catalogLink is the catalog's row userID's food id names, or "".
func catalogLink(t *testing.T, db *store.DB, userID, id uuid.UUID) string {
	t.Helper()
	var link sql.NullString
	if err := db.Read().QueryRowContext(t.Context(), "SELECT catalog_id FROM foods WHERE user_id = ? AND id = ?",
		userID.String(), id.String()).Scan(&link); err != nil {
		t.Fatal(err)
	}
	return link.String
}

// nothingOf fails the test when userID has a food of any of names.
func nothingOf(t *testing.T, db *store.DB, userID uuid.UUID, names ...string) {
	t.Helper()
	for _, n := range names {
		if f, ok := named(t, db, userID, n); ok {
			t.Errorf("%v's %s = %+v, want none", userID, n, f)
		}
	}
}

// A food a member saves is each member's at once, under their own id, of
// its shared fields, barcodes too, but none of the saver's own words; a
// scan finds it; one catalog row is the copies'. A user outside the
// household has none of it.
func TestHouseholdSharesANewFood(t *testing.T) {
	t.Parallel()

	db, ana, ivo, eva := withHousehold(t)
	anas := saveFood(t, db, ana, tuna(116))
	ivos := copyNamed(t, db, ivo, anas.Name)
	if ivos.ID == anas.ID || !sameShared(ivos, anas) || ivos.Note != "" || len(ivos.Aliases) != 0 {
		t.Errorf("Ivo's tuna = %+v, want Ana's %+v under his own id, without her words", ivos, anas)
	}
	if got, err := db.FoodByBarcode(t.Context(), ivo, "3850104000010"); err != nil || got.ID != ivos.ID {
		t.Errorf("Ivo's scan = %+v, %v; want his tuna", got, err)
	}
	if a, i := catalogLink(t, db, ana, anas.ID), catalogLink(t, db, ivo, ivos.ID); a == "" || a != i || catalogRows(t, db) != 1 {
		t.Errorf("the catalog's rows of the tuna: Ana's %q, Ivo's %q, of %d rows; want one, the same", a, i, catalogRows(t, db))
	}
	nothingOf(t, db, eva, anas.Name)
	if _, err := db.FoodByBarcode(t.Context(), eva, "3850104000010"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Eva's scan: %v, want nothing", err)
	}

	// A food made by CreateFood (the web app's, and "Add to my foods") is
	// the household's too.
	if _, err := db.CreateFood(t.Context(), ivo, testActor, kefir(), t0); err != nil {
		t.Fatal(err)
	}
	copyNamed(t, db, ana, "Kefir")
	nothingOf(t, db, eva, "Kefir")
}

// A correction reaches every member's copy; each member's note, aliases
// and archived stay theirs, and so does an alias of theirs a search finds.
func TestHouseholdCorrectionKeepsEachMembersOwn(t *testing.T) {
	t.Parallel()

	db, ana, ivo, eva := withHousehold(t)
	anas := saveFood(t, db, ana, bar())
	ivos := copyNamed(t, db, ivo, anas.Name)
	ivos.Note, ivos.Aliases = "Ivo's words", []string{"gym bar"}
	ivos = saveFood(t, db, ivo, ivos)
	if err := db.SetFoodArchived(t.Context(), ivo, testActor, ivos.ID, true, t0); err != nil {
		t.Fatal(err)
	}
	evas := saveFood(t, db, eva, bar())

	anas = copyNamed(t, db, ana, anas.Name)
	if anas.Note != "from the label" || !slices.Equal(anas.Aliases, []string{"caramel bar"}) || anas.Archived {
		t.Errorf("Ana's bar after Ivo's save = %+v, want her words, in use", anas)
	}
	anas.Name, anas.Nutrients.Kcal, anas.Barcodes = "Protein bar Salted caramel", 370, []string{"7340001802009"}
	anas, err := db.CorrectFood(t.Context(), ana, testActor, anas, t0)
	if err != nil {
		t.Fatal(err)
	}
	ownWordsKept(t, db, ivo, ivos.ID, anas)
	if found, err := db.FindArchivedFoods(t.Context(), ivo, "gym", 5); err != nil || len(found) != 1 || found[0].ID != ivos.ID {
		t.Errorf("Ivo's search by his alias = %v, %v; want his bar", names(found), err)
	}
	if got, err := db.FoodByID(t.Context(), eva, evas.ID); err != nil || got.Name != bar().Name || got.Nutrients.Kcal != bar().Nutrients.Kcal {
		t.Errorf("Eva's bar = %+v, %v; want it as she saved it", got, err)
	}
}

// ownWordsKept fails the test unless Ivo's bar id is Ana's anas, with
// his note and aliases, archived.
func ownWordsKept(t *testing.T, db *store.DB, ivo, id uuid.UUID, anas food.Food) {
	t.Helper()
	got, err := db.FoodByID(t.Context(), ivo, id)
	if err != nil || !sameShared(got, anas) || got.Note != "Ivo's words" || !slices.Equal(got.Aliases, []string{"gym bar"}) || !got.Archived {
		t.Errorf("Ivo's bar after Ana's correction = %+v, %v; want her values, his words, archived", got, err)
	}
}

// The new foods of a meal, logged or corrected, are the household's; so
// is a barcode a log links to a food a member has, and the catalog's food
// a log copies, which keeps its row.
func TestHouseholdFoodsOfAMeal(t *testing.T) {
	t.Parallel()

	db, ana, ivo, eva := withHousehold(t)
	b := saveFood(t, db, ana, bar())
	k := kefir()
	m := breakfast(t, b, t0)
	m.Items = append(m.Items, itemOf(t, k, meal.Quantity{Amount: 250, Unit: "ml"}))
	if _, _, err := db.CreateMealWithFoods(t.Context(), ana, testActor, m, []food.Food{k}, t0); err != nil {
		t.Fatalf("CreateMealWithFoods: %v", err)
	}
	copyNamed(t, db, ivo, "Kefir")

	// A food of the bar's name with a barcode is the bar, scanned.
	scanned := withBarcode(bar(), "7340001802009")
	tea := chickenBreast()
	tea.Name = "Green tea"
	if _, err := db.UpdateMeal(t.Context(), ana, testActor, m.ID, []food.Food{scanned, tea}, func(m meal.Meal) (meal.Meal, error) {
		m.Items = append(m.Items, itemOf(t, scanned, meal.Quantity{Servings: 1, Serving: "bar"}), itemOf(t, tea, meal.Quantity{Amount: 300, Unit: "g"}))
		return m, nil
	}, t0); err != nil {
		t.Fatalf("UpdateMeal: %v", err)
	}
	copyNamed(t, db, ivo, "Green tea")
	if got, err := db.FoodByBarcode(t.Context(), ivo, "7340001802009"); err != nil || got.Name != b.Name {
		t.Errorf("Ivo's scan of the barcode Ana's log linked = %+v, %v; want his bar", got, err)
	}

	// Ivo copies the catalog's tuna on a log of his.
	saveFood(t, db, eva, tuna(116))
	row, err := db.CatalogFoodByBarcode(t.Context(), "3850104000010")
	if err != nil {
		t.Fatal(err)
	}
	cp := row
	cp.ID, cp.CopyOf = uuid.NewV7(), row.ID
	lunch := breakfast(t, b, t0.Add(4*time.Hour))
	lunch.Items = []meal.Item{itemOf(t, cp, meal.Quantity{Amount: 80, Unit: "g"})}
	if _, _, err := db.CreateMealWithFoods(t.Context(), ivo, testActor, lunch, []food.Food{cp}, t0); err != nil {
		t.Fatalf("CreateMealWithFoods: %v", err)
	}
	anas := copyNamed(t, db, ana, row.Name)
	if link := catalogLink(t, db, ana, anas.ID); link != row.ID.String() {
		t.Errorf("Ana's copy of Ivo's catalog tuna names row %q, want %s", link, row.ID)
	}
	nothingOf(t, db, eva, "Kefir", "Green tea")
}

// A recipe a member saves is each member's, of each member's own copies
// of its ingredients, with its barcodes; a member's correction, a dish
// cooked again, is the others' too, of their own ingredients still.
func TestHouseholdSharesARecipe(t *testing.T) {
	t.Parallel()

	db, ana, ivo, eva := withHousehold(t)
	chicken := chickenBreast()
	k := kefir()
	r := curryOf(t, chicken, k)
	r.Food.Barcodes = []string{"2000000000015"}
	anas := saveRecipe(t, db, ana, r, chicken, k)
	ivos := recipeOf(t, db, ivo)
	ownIngredients(t, db, ivo, ivos)
	if !sameDish(ivos, anas) || !slices.Equal(ivos.Food.Barcodes, anas.Food.Barcodes) || len(ivos.Food.Aliases) != 0 {
		t.Errorf("Ivo's curry = %+v, want Ana's dish and barcodes, without her aliases", ivos)
	}

	ivos.Yield, ivos.Portions = 1200, 5
	ivos.Food.Aliases = []string{"Ivo's curry"}
	if _, err := db.CorrectRecipe(t.Context(), ivo, testActor, ivos, nil, t0); err != nil {
		t.Fatal(err)
	}
	got := recipeOf(t, db, ana)
	ownIngredients(t, db, ana, got)
	if got.Yield != 1200 || got.Portions != 5 || !slices.Equal(got.Food.Aliases, []string{"the curry"}) {
		t.Errorf("Ana's curry after Ivo cooked it again = %+v, want his dish, her aliases", got)
	}
	nothingOf(t, db, eva, "Chicken curry", "Chicken breast", "Kefir")
}

// ownIngredients fails the test when an ingredient of userID's recipe r
// is not of a food of their own.
func ownIngredients(t *testing.T, db *store.DB, userID uuid.UUID, r recipe.Recipe) {
	t.Helper()
	for i := range r.Items {
		it := &r.Items[i]
		f, err := db.FoodByID(t.Context(), userID, it.FoodID)
		if it.FoodUser != uuid.Nil() || err != nil || f.Name != it.Name {
			t.Errorf("%v's ingredient %q is of %v's food %v (%v), want their own", userID, it.Name, it.FoodUser, it.FoodID, err)
		}
	}
}

// A recipe of foods of before the household brings the household its
// ingredients first, of which each member's recipe is made.
func TestHouseholdRecipeOfAFoodOfBefore(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	chicken := saveFood(t, db, ana, chickenBreast())
	inHousehold(t, db, ana, ivo)
	k := kefir()
	saveRecipe(t, db, ana, curryOf(t, chicken, k), k)
	ownIngredients(t, db, ivo, recipeOf(t, db, ivo))
	copyNamed(t, db, ivo, chicken.Name)
}

// A save a member's foods clash with, a food of the name or the barcode
// that is no copy of the saver's, is refused, writing nothing: which food
// is theirs alone to know.
func TestHouseholdRefusesAMembersClash(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	// Ivo's foods of before the household, which no merge brought Ana.
	saveFood(t, db, ivo, bar())
	saveFood(t, db, ivo, withBarcode(chickenBreast(), "2000000000015"))
	inHousehold(t, db, ana, ivo)

	if _, err := db.SaveFood(t.Context(), ana, testActor, bar(), t0); !errors.Is(err, store.ErrShareNameTaken) {
		t.Errorf("Ana's bar of Ivo's bar's name = %v, want ErrShareNameTaken", err)
	}
	k := withBarcode(kefir(), "2000000000015")
	if _, err := db.SaveFood(t.Context(), ana, testActor, k, t0); !errors.Is(err, store.ErrShareBarcodeTaken) {
		t.Errorf("Ana's kefir of Ivo's chicken's barcode = %v, want ErrShareBarcodeTaken", err)
	}
	nothingOf(t, db, ana, bar().Name, "Kefir")
	nothingOf(t, db, ivo, "Kefir")
}

// The undo of a member's save takes it back for every member: a new
// food goes from each, a correction comes back in each copy, each
// member's own words kept.
func TestHouseholdUndo(t *testing.T) {
	t.Parallel()

	db, ana, ivo, _ := withHousehold(t)
	anas := saveFood(t, db, ana, bar())
	ivos := copyNamed(t, db, ivo, anas.Name)
	ivos.Note = "Ivo's words"
	saveFood(t, db, ivo, ivos)
	// The latest save is Ivo's: Ana's undo of hers is refused.
	if _, err := db.UndoLast(t.Context(), ana, testActor, t0.Add(time.Minute)); !isConflict(err, store.UndoChanged) {
		t.Fatalf("Ana's undo after Ivo's save = %v, want it changed since", err)
	}

	corrected := copyNamed(t, db, ana, anas.Name)
	corrected.Nutrients.Kcal = 400
	saveFood(t, db, ana, corrected)
	undoAt(t, db, ana, 2*time.Minute)
	if got := copyNamed(t, db, ivo, anas.Name); got.Nutrients.Kcal != bar().Nutrients.Kcal || got.Note != "Ivo's words" {
		t.Errorf("Ivo's bar after Ana's undo = %+v, want it as before, his words kept", got)
	}

	k := saveFood(t, db, ana, kefir())
	undoAt(t, db, ana, 3*time.Minute)
	nothingOf(t, db, ana, k.Name)
	nothingOf(t, db, ivo, k.Name)
}

// isConflict reports whether err is an undo refused for reason.
func isConflict(err error, reason store.UndoReason) bool {
	c, ok := errors.AsType[*store.UndoConflictError](err)
	return ok && c.Reason == reason
}

// A new food's undo keeps the copy a member's meal uses, archived.
func TestHouseholdUndoKeepsACopyInUse(t *testing.T) {
	t.Parallel()

	db, ana, ivo, _ := withHousehold(t)
	anas := saveFood(t, db, ana, bar())
	// Ivo's meal comes after Ana's save; her undo is of her own latest
	// change, the save, which the meal does not touch.
	ivos := copyNamed(t, db, ivo, anas.Name)
	if _, _, err := db.CreateMeal(t.Context(), ivo, testActor, breakfast(t, ivos, t0), t0); err != nil {
		t.Fatal(err)
	}
	undoAt(t, db, ana, time.Minute)
	nothingOf(t, db, ana, anas.Name)
	if got, err := db.FoodByID(t.Context(), ivo, ivos.ID); err != nil || !got.Archived {
		t.Errorf("Ivo's bar after Ana's undo = %+v, %v; want it kept for his meal, archived", got, err)
	}
}

// A recipe's undo takes it back for every member, and is refused after
// another member's save.
func TestHouseholdUndoRecipe(t *testing.T) {
	t.Parallel()

	db, ana, ivo, _ := withHousehold(t)
	chicken := saveFood(t, db, ana, chickenBreast())
	k := saveFood(t, db, ana, kefir())
	anas := saveRecipe(t, db, ana, curryOf(t, chicken, k))
	anas.Yield = 1100
	saveRecipe(t, db, ana, anas)
	undoAt(t, db, ana, time.Minute)
	if got := recipeOf(t, db, ivo); got.Yield != 1000 {
		t.Errorf("Ivo's curry after Ana's undo weighs %v, want 1000 again", got.Yield)
	}

	ivos := recipeOf(t, db, ivo)
	ivos.Portions = 6
	saveRecipe(t, db, ivo, ivos)
	if _, err := db.UndoLast(t.Context(), ana, testActor, t0.Add(2*time.Minute)); !isConflict(err, store.UndoChanged) {
		t.Fatalf("Ana's undo after Ivo's save = %v, want it changed since", err)
	}

	db2, ana2, ivo2, _ := withHousehold(t)
	c2 := saveFood(t, db2, ana2, chickenBreast())
	k2 := saveFood(t, db2, ana2, kefir())
	saveRecipe(t, db2, ana2, curryOf(t, c2, k2))
	undoAt(t, db2, ana2, time.Minute)
	nothingOf(t, db2, ana2, "Chicken curry")
	nothingOf(t, db2, ivo2, "Chicken curry")
}

// A recipe shared with a member by a user outside the household is every
// member's; its undo takes it from them all. A stop of it takes it from
// the household whole; a member never stops another's, and a household's
// recipe of its own is never left by it.
func TestShareWithAHousehold(t *testing.T) {
	t.Parallel()

	db, ana, ivo, eva := withHousehold(t)
	chicken := saveFood(t, db, eva, chickenBreast())
	k := kefir()
	evas := saveRecipe(t, db, eva, curryOf(t, chicken, k), k)
	if err := db.ShareRecipe(t.Context(), eva, testActor, evas.Food.ID, ana, t0); err != nil {
		t.Fatal(err)
	}
	ivos := recipeOf(t, db, ivo)
	if !sameDish(ivos, evas) {
		t.Errorf("Ivo's curry = %+v, want Eva's dish", ivos)
	}
	undoAt(t, db, eva, time.Minute)
	nothingOf(t, db, ana, "Chicken curry")
	nothingOf(t, db, ivo, "Chicken curry")

	if err := db.ShareRecipe(t.Context(), eva, testActor, evas.Food.ID, ivo, t0.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	anas := recipeOf(t, db, ana).Food.ID
	if err := db.UnshareRecipe(t.Context(), ana, testActor, anas, seatOf(t, db, ana, anas, ivo), t0); !errors.Is(err, store.ErrShareHousehold) {
		t.Errorf("Ana's stop of Ivo's share = %v, want ErrShareHousehold", err)
	}
	if err := db.UnshareRecipe(t.Context(), eva, testActor, evas.Food.ID, seatOf(t, db, eva, evas.Food.ID, ana), t0); err != nil {
		t.Fatal(err)
	}
	if members, err := db.RecipeMembers(t.Context(), eva, evas.Food.ID); err != nil || members != nil {
		t.Errorf("the curry's members after Eva stopped Ana's share = %+v, %v; want none: the household went", members, err)
	}
	undoAt(t, db, eva, 3*time.Minute)
	if members, err := db.RecipeMembers(t.Context(), eva, evas.Food.ID); err != nil || len(members) != 3 {
		t.Errorf("the curry's members after the undo = %+v, %v; want the household back", members, err)
	}

	stew := curryOf(t, saveFood(t, db, ana, chickenBreast()), saveFood(t, db, ana, kefir()))
	stew.Food.Name = "Stew"
	stew = saveRecipe(t, db, ana, stew)
	if err := db.UnshareRecipe(t.Context(), ana, testActor, stew.Food.ID, seatOf(t, db, ana, stew.Food.ID, ana), t0); !errors.Is(err, store.ErrShareHousehold) {
		t.Errorf("Ana's leave of the household's stew = %v, want ErrShareHousehold", err)
	}
}

// A user whose recipe a household has deletes their account: each member
// gets one food of each ingredient of theirs, the household's, of which
// each member's recipe is made then.
func TestHouseholdAdoptsAGoneUsersIngredients(t *testing.T) {
	t.Parallel()

	db, ana, ivo, eva := withHousehold(t)
	chicken := saveFood(t, db, eva, chickenBreast())
	k := kefir()
	evas := saveRecipe(t, db, eva, curryOf(t, chicken, k), k)
	if err := db.ShareRecipe(t.Context(), eva, testActor, evas.Food.ID, ana, t0); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DeleteUser(t.Context(), eva); err != nil {
		t.Fatal(err)
	}
	for _, u := range []uuid.UUID{ana, ivo} {
		ownIngredients(t, db, u, recipeOf(t, db, u))
		nothingOf(t, db, u, "Chicken breast (2)", "Kefir (2)")
	}
}
