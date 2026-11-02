package store_test

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/food"
	"github.com/zarnautovic/tdeemon/internal/domain/meal"
	"github.com/zarnautovic/tdeemon/internal/domain/recipe"
	"github.com/zarnautovic/tdeemon/internal/store"
)

// householdHolds fails the test unless members keep one kitchen, as a
// household does (story 408): they are of one household; each food of a
// member's, but a recipe they left, is in a share group with a live seat
// of each other member, at one food of theirs; every copy has the same
// shared fields and catalog row, and a recipe's copies the same dish,
// each ingredient of a food of the kitchen's of the member's own copy.
func householdHolds(t *testing.T, db *store.DB, members ...uuid.UUID) {
	t.Helper()
	if got := membersOf(t, db, members[0]); !slices.Equal(got, sortedIDs(members)) {
		t.Fatalf("the household of %v = %v, want them all", members[0], got)
	}
	count := -1
	for _, m := range members {
		ids := liveFoods(t, db, m)
		if count >= 0 && len(ids) != count {
			t.Errorf("%v has %d foods, another member %d", m, len(ids), count)
		}
		count = len(ids)
		for _, id := range ids {
			group := liveGroup(t, db, m, id)
			if group == "" {
				t.Errorf("%v's food %v is in no group", m, id)
				continue
			}
			for _, o := range members {
				if o != m {
					sameCopies(t, db, members, group, shareSeat{m, id}, o)
				}
			}
		}
	}
}

// shareSeat is a member's food.
type shareSeat struct{ user, food uuid.UUID }

// sameCopies fails the test unless member o has a copy of mine in group,
// of its shared fields, catalog row and dish.
func sameCopies(t *testing.T, db *store.DB, members []uuid.UUID, group string, mine shareSeat, o uuid.UUID) {
	t.Helper()
	var s string
	if err := db.Read().QueryRowContext(t.Context(), "SELECT food_id FROM food_shares WHERE share_id = ? AND user_id = ? AND left_at IS NULL",
		group, o.String()).Scan(&s); err != nil {
		t.Errorf("%v has no seat in the group of %v's food %v: %v", o, mine.user, mine.food, err)
		return
	}
	theirs := shareSeat{o, uuid.MustParse(s)}
	a, b := foodAt(t, db, mine), foodAt(t, db, theirs)
	if !sameShared(a, b) {
		t.Errorf("%v's %+v and %v's copy %+v differ", mine.user, a, o, b)
	}
	if x, y := catalogLink(t, db, mine.user, mine.food), catalogLink(t, db, o, theirs.food); x != y {
		t.Errorf("%v's %s names catalog row %q, %v's copy %q", mine.user, a.Name, x, o, y)
	}
	if a.Source != food.Recipe {
		return
	}
	ra, rb := recipeAt(t, db, mine), recipeAt(t, db, theirs)
	if !sameDish(ra, rb) {
		t.Errorf("%v's recipe %s and %v's differ: %+v, %+v", mine.user, a.Name, o, ra, rb)
		return
	}
	for i := range ra.Items {
		if x, y := ingredientKey(t, db, members, mine.user, ra.Items[i]), ingredientKey(t, db, members, o, rb.Items[i]); x != y {
			t.Errorf("%s's ingredient %d: %v's is of %s, %v's of %s", a.Name, i, mine.user, x, o, y)
		}
	}
}

// ingredientKey names the food of userID's ingredient it: a group of the
// kitchen's, or a food outside it by its owner, or none.
func ingredientKey(t *testing.T, db *store.DB, members []uuid.UUID, userID uuid.UUID, it meal.Item) string {
	t.Helper()
	switch {
	case it.FoodID == uuid.Nil():
		return "none"
	case it.FoodUser == uuid.Nil():
		return liveGroup(t, db, userID, it.FoodID)
	case slices.Contains(members, it.FoodUser) && !leftBy(t, db, it.FoodUser, it.FoodID):
		t.Errorf("%v's ingredient %s is of %v's food, a member's", userID, it.Name, it.FoodUser)
	}
	return it.FoodUser.String() + "/" + it.FoodID.String()
}

func sortedIDs(ids []uuid.UUID) []uuid.UUID {
	out := slices.Clone(ids)
	slices.SortFunc(out, uuid.UUID.Compare)
	return out
}

// liveFoods lists userID's foods but the recipes they left.
func liveFoods(t *testing.T, db *store.DB, userID uuid.UUID) []uuid.UUID {
	t.Helper()
	rows, err := db.Read().QueryContext(t.Context(), `SELECT id FROM foods AS f WHERE user_id = ?
		AND NOT EXISTS (SELECT 1 FROM food_shares AS s WHERE s.user_id = f.user_id AND s.food_id = f.id AND s.left_at IS NOT NULL)`,
		userID.String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []uuid.UUID
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, uuid.MustParse(s))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// liveGroup is the live share group of userID's food id, or "".
func liveGroup(t *testing.T, db *store.DB, userID, id uuid.UUID) string {
	t.Helper()
	var s string
	err := db.Read().QueryRowContext(t.Context(), "SELECT share_id FROM food_shares WHERE user_id = ? AND food_id = ? AND left_at IS NULL",
		userID.String(), id.String()).Scan(&s)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	return s
}

// leftBy reports whether userID's food id is a recipe they left.
func leftBy(t *testing.T, db *store.DB, userID, id uuid.UUID) bool {
	t.Helper()
	var left bool
	if err := db.Read().QueryRowContext(t.Context(), "SELECT EXISTS (SELECT 1 FROM food_shares WHERE user_id = ? AND food_id = ? AND left_at IS NOT NULL)",
		userID.String(), id.String()).Scan(&left); err != nil {
		t.Fatal(err)
	}
	return left
}

func foodAt(t *testing.T, db *store.DB, s shareSeat) food.Food {
	t.Helper()
	f, err := db.FoodByID(t.Context(), s.user, s.food)
	if err != nil {
		t.Fatalf("%v's food %v: %v", s.user, s.food, err)
	}
	return f
}

func recipeAt(t *testing.T, db *store.DB, s shareSeat) recipe.Recipe {
	t.Helper()
	r, err := db.RecipeByID(t.Context(), s.user, s.food)
	if err != nil {
		t.Fatalf("%v's recipe %v: %v", s.user, s.food, err)
	}
	return r
}

// kitchenDump is userID's foods and recipes as rows of text: what a
// merge they are no part of must leave as it was.
func kitchenDump(t *testing.T, db *store.DB, userID uuid.UUID) string {
	t.Helper()
	var b strings.Builder
	for _, q := range []string{
		`SELECT id || name || brand || kcal || note || archived || updated_at || coalesce(catalog_id, '') FROM foods WHERE user_id = ? ORDER BY id`,
		"SELECT food_id || code FROM food_barcodes WHERE user_id = ? ORDER BY food_id, code",
		"SELECT recipe_id || position || coalesce(food_user_id, '') || coalesce(food_id, '') || amount FROM recipe_items WHERE user_id = ? ORDER BY recipe_id, position",
	} {
		for _, s := range textRows(t, db, q, userID.String()) {
			fmt.Fprintln(&b, s)
		}
	}
	return b.String()
}

// textRows is the rows of q, of one text column each.
func textRows(t *testing.T, db *store.DB, q string, args ...any) []string {
	t.Helper()
	rows, err := db.Read().QueryContext(t.Context(), q, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// renamed returns f under a new id, of name.
func renamed(f food.Food, name string) food.Food {
	f.ID, f.Name = uuid.NewV7(), name
	return f
}

// A food both sides have is one, of the inviter's values: the invited
// user's keeps its id, which their meal names, their note, aliases and
// archived; its barcodes are both sides'. What one side lacks it gets.
// A user outside is never written.
func TestMergeTheInviterWins(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	eva := addEva(t, db)
	anas := saveFood(t, db, ana, withBarcode(bar(), "7340001802009"))
	mine := bar()
	mine.Nutrients.Kcal, mine.Note, mine.Aliases = 300, "Ivo's words", []string{"gym bar"}
	mine = saveFood(t, db, ivo, withBarcode(mine, "7340001802016"))
	if err := db.SetFoodArchived(t.Context(), ivo, testActor, mine.ID, true, t0); err != nil {
		t.Fatal(err)
	}
	lunch := createMeal(t, db, ivo, breakfast(t, mine, t0))
	saveFood(t, db, ana, kefir())
	saveFood(t, db, ivo, chickenBreast())
	saveFood(t, db, eva, bar())
	saveFood(t, db, eva, kefir())
	evas := kitchenDump(t, db, eva)

	got := join(t, db, ana, ivo)
	want := store.MergeReport{Merged: store.MergeCounts{Foods: 1}, Received: store.MergeCounts{Foods: 1}, Given: store.MergeCounts{Foods: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("report = %+v, want %+v", got, want)
	}
	ivos := foodAt(t, db, shareSeat{ivo, mine.ID})
	if !reflect.DeepEqual(ivos.Nutrients, anas.Nutrients) || ivos.Note != "Ivo's words" || !slices.Equal(ivos.Aliases, []string{"gym bar"}) || !ivos.Archived {
		t.Errorf("Ivo's bar = %+v, want Ana's values under his id, his words, archived", ivos)
	}
	codes := []string{"7340001802009", "7340001802016"}
	if !slices.Equal(ivos.Barcodes, codes) || !slices.Equal(copyNamed(t, db, ana, anas.Name).Barcodes, codes) {
		t.Errorf("the bar's barcodes = %v, want both sides'", ivos.Barcodes)
	}
	if m, err := db.MealByID(t.Context(), ivo, lunch.ID); err != nil || m.Items[0].FoodID != mine.ID {
		t.Errorf("Ivo's meal = %+v, %v; want it of his bar still", m, err)
	}
	copyNamed(t, db, ivo, "Kefir")
	copyNamed(t, db, ana, "Chicken breast")
	// The household's from then on: Ivo's correction is Ana's too.
	ivos.Nutrients.Kcal = 370
	saveFood(t, db, ivo, ivos)
	if f := copyNamed(t, db, ana, anas.Name); f.Nutrients.Kcal != 370 || f.Note != anas.Note {
		t.Errorf("Ana's bar after Ivo's correction = %+v, want his values, her words", f)
	}
	householdHolds(t, db, ana, ivo)
	if kitchenDump(t, db, eva) != evas {
		t.Errorf("Eva's kitchen changed:\n%s\nwas\n%s", kitchenDump(t, db, eva), evas)
	}
}

// Two foods of no name in common match by a barcode, else by the
// catalog's row, the most recently saved first; the invited user's takes
// the inviter's name.
func TestMergeMatchesByBarcodeThenCatalogRow(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	eva := addEva(t, db)
	saveFood(t, db, ana, withBarcode(kefir(), "2000000000015"))
	ivosKefir := saveFood(t, db, ivo, withBarcode(renamed(kefir(), "Kefir natur"), "2000000000015"))

	tunas := saveFood(t, db, eva, tuna(116))
	row, err := db.CatalogFoodByBarcode(t.Context(), "3850104000010")
	if err != nil {
		t.Fatal(err)
	}
	saveFood(t, db, ana, tuna(116))
	// Two of Ivo's foods are copies of the row, neither of its name.
	copyAs := func(name string, at time.Time) food.Food {
		cp := renamed(row, name)
		cp.CopyOf, cp.Barcodes = row.ID, nil
		got, err := db.SaveFood(t.Context(), ivo, testActor, cp, at)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	newer := copyAs("My tuna", t0.Add(time.Minute))
	older := copyAs("Old tuna", t0)

	got := join(t, db, ana, ivo)
	if got.Merged.Foods != 2 || got.Given.Foods != 1 {
		t.Errorf("report = %+v, want 2 merged and Ivo's older tuna given", got)
	}
	if f := foodAt(t, db, shareSeat{ivo, ivosKefir.ID}); f.Name != "Kefir" {
		t.Errorf("Ivo's kefir is %q, want Ana's name", f.Name)
	}
	if f := foodAt(t, db, shareSeat{ivo, newer.ID}); f.Name != tunas.Name {
		t.Errorf("Ivo's newer tuna is %q, want Ana's tuna", f.Name)
	}
	if f := foodAt(t, db, shareSeat{ivo, older.ID}); f.Name != "Old tuna" {
		t.Errorf("Ivo's older tuna is %q, want it as it was", f.Name)
	}
	copyNamed(t, db, ana, "Old tuna")
}

// A barcode another food of the merged kitchen has goes from the invited
// user's food, which keeps the rest; the report names each.
func TestMergeDropsAClashingBarcode(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	saveFood(t, db, ana, withBarcode(kefir(), "2000000000015"))
	saveFood(t, db, ana, withBarcode(chickenBreast(), "2000000000022"))
	// Ivo's kefir is Ana's, of a barcode her chicken has; his yoghurt has
	// the barcode of her kefir.
	mine := kefir()
	mine.Barcodes = []string{"2000000000022", "2000000000039"}
	saveFood(t, db, ivo, mine)
	yoghurt := saveFood(t, db, ivo, withBarcode(renamed(kefir(), "Yoghurt"), "2000000000015"))

	got := join(t, db, ana, ivo)
	want := []store.MergeBarcode{{Food: "Kefir", Code: "2000000000022"}, {Food: "Yoghurt", Code: "2000000000015"}}
	if !reflect.DeepEqual(got.Dropped, want) {
		t.Errorf("dropped = %+v, want %+v", got.Dropped, want)
	}
	if f := copyNamed(t, db, ivo, "Kefir"); !slices.Equal(f.Barcodes, []string{"2000000000015", "2000000000039"}) {
		t.Errorf("the kefir's barcodes = %v, want Ana's and Ivo's of no other food", f.Barcodes)
	}
	if f := foodAt(t, db, shareSeat{ivo, yoghurt.ID}); len(f.Barcodes) != 0 {
		t.Errorf("Ivo's yoghurt's barcodes = %v, want none", f.Barcodes)
	}
	if f, err := db.FoodByBarcode(t.Context(), ivo, "2000000000015"); err != nil || f.Name != "Kefir" {
		t.Errorf("Ivo's scan of the kefir = %+v, %v", f, err)
	}
}

// A match keeps as many barcodes as a food may have, the inviter's
// first; the invited user's past them are dropped.
func TestMergeKeepsTheBarcodesAFoodMayHave(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	theirs, mine := kefir(), kefir()
	for i := range 6 {
		theirs.Barcodes = append(theirs.Barcodes, ean13(t, 2000000000+i))
		mine.Barcodes = append(mine.Barcodes, ean13(t, 2000000100+i))
	}
	saveFood(t, db, ana, theirs)
	saveFood(t, db, ivo, mine)

	got := join(t, db, ana, ivo)
	want := []store.MergeBarcode{{Food: "Kefir", Code: mine.Barcodes[4]}, {Food: "Kefir", Code: mine.Barcodes[5]}}
	if !reflect.DeepEqual(got.Dropped, want) {
		t.Errorf("dropped = %+v, want %+v", got.Dropped, want)
	}
	if f := copyNamed(t, db, ivo, "Kefir"); !slices.Equal(f.Barcodes, append(theirs.Barcodes, mine.Barcodes[:4]...)) {
		t.Errorf("the kefir's barcodes = %v, want Ana's six and Ivo's first four", f.Barcodes)
	}
}

// A recipe and a food of one name do not merge: the invited user's is
// renamed, of the lowest number free.
func TestMergeRenamesARecipeOfAFoodsName(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	plate := chickenBreast()
	plate.Name, plate.Source = "Chicken curry", food.Estimate
	saveFood(t, db, ana, plate)
	saveFood(t, db, ana, renamed(plate, "Chicken curry (2)"))
	chicken := saveFood(t, db, ivo, chickenBreast())
	k := kefir()
	ivos := saveRecipe(t, db, ivo, curryOf(t, chicken, k), k)

	got := join(t, db, ana, ivo)
	if want := []store.MergeRename{{From: "Chicken curry", To: "Chicken curry (3)"}}; !reflect.DeepEqual(got.Renamed, want) {
		t.Errorf("renamed = %+v, want %+v", got.Renamed, want)
	}
	if got.Given.Recipes != 1 {
		t.Errorf("report = %+v, want Ivo's recipe given", got)
	}
	if f := foodAt(t, db, shareSeat{ivo, ivos.Food.ID}); f.Name != "Chicken curry (3)" || f.Source != food.Recipe {
		t.Errorf("Ivo's curry = %+v, want it renamed", f)
	}
	if f := copyNamed(t, db, ana, "Chicken curry (3)"); f.Source != food.Recipe {
		t.Errorf("Ana's copy of Ivo's curry = %+v, want a recipe", f)
	}
}

// Two recipes merge of the inviter's dish, each of the member's own
// copies of its ingredients; a recipe one side lacks comes to it, of its
// own copies too.
func TestMergeRecipes(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	anas := saveRecipe(t, db, ana, curryOf(t, saveFood(t, db, ana, chickenBreast()), saveFood(t, db, ana, kefir())))
	ivosChicken := saveFood(t, db, ivo, chickenBreast())
	ivosKefir := saveFood(t, db, ivo, kefir())
	mine := curryOf(t, ivosChicken, ivosKefir)
	mine.Yield, mine.Portions = 900, 3
	mine.Items = mine.Items[:1]
	mine = saveRecipe(t, db, ivo, mine)
	beef := chickenBreast()
	beef.Name = "Beef"
	stew := curryOf(t, saveFood(t, db, ana, beef), saveFood(t, db, ana, renamed(kefir(), "Onion")))
	stew.Food.Name, stew.Food.Aliases = "Stew", nil
	saveRecipe(t, db, ana, stew)
	carrot := saveFood(t, db, ivo, renamed(chickenBreast(), "Carrot"))
	soup := curryOf(t, carrot, ivosKefir)
	soup.Food.Name, soup.Food.Aliases = "Soup", nil
	saveRecipe(t, db, ivo, soup)

	got := join(t, db, ana, ivo)
	want := store.MergeReport{
		Merged:   store.MergeCounts{Foods: 2, Recipes: 1},
		Received: store.MergeCounts{Foods: 2, Recipes: 1},
		Given:    store.MergeCounts{Foods: 1, Recipes: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("report = %+v, want %+v", got, want)
	}
	r := recipeAt(t, db, shareSeat{ivo, mine.Food.ID})
	if !sameDish(r, anas) || len(r.Items) != 2 || r.Items[0].FoodID != ivosChicken.ID || r.Items[1].FoodID != ivosKefir.ID {
		t.Errorf("Ivo's curry = %+v, want Ana's dish of his own chicken and kefir", r)
	}
	for _, u := range []uuid.UUID{ana, ivo} {
		for _, name := range []string{"Stew", "Soup", "Chicken curry"} {
			ownIngredients(t, db, u, recipeAt(t, db, shareSeat{u, copyNamed(t, db, u, name).ID}))
		}
	}
}

// A third user joins a household of two through any member's invite:
// the household's foods are theirs, and theirs the household's.
func TestMergeAThirdMember(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	eva := addEva(t, db)
	saveFood(t, db, ana, bar())
	saveFood(t, db, ivo, kefir())
	join(t, db, ana, ivo)
	mine := bar()
	mine.Nutrients.Kcal = 1
	saveFood(t, db, eva, mine)
	saveFood(t, db, eva, chickenBreast())

	got := join(t, db, ivo, eva)
	want := store.MergeReport{Merged: store.MergeCounts{Foods: 1}, Received: store.MergeCounts{Foods: 1}, Given: store.MergeCounts{Foods: 1}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("report = %+v, want %+v", got, want)
	}
	if len(membersOf(t, db, ana)) != 3 {
		t.Errorf("Ana's household = %v, want three", membersOf(t, db, ana))
	}
	if f := copyNamed(t, db, eva, bar().Name); f.Nutrients.Kcal != bar().Nutrients.Kcal {
		t.Errorf("Eva's bar = %+v, want the household's values", f)
	}
}

// A recipe of the invited user's shared with a user outside keeps its
// group, which the household joins; one that matched a recipe of the
// household's leaves that group for the household's, whose outside
// member keeps hers.
func TestMergeARecipeSharedOutside(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	eva := addEva(t, db)
	soup := curryOf(t, saveFood(t, db, ivo, chickenBreast()), saveFood(t, db, ivo, kefir()))
	soup.Food.Name, soup.Food.Aliases = "Soup", nil
	soup = saveRecipe(t, db, ivo, soup)
	curry := saveRecipe(t, db, ivo, curryOf(t, copyNamed(t, db, ivo, "Chicken breast"), copyNamed(t, db, ivo, "Kefir")))
	for _, r := range []recipe.Recipe{soup, curry} {
		if err := db.ShareRecipe(t.Context(), ivo, testActor, r.Food.ID, eva, t0); err != nil {
			t.Fatal(err)
		}
	}
	anas := curryOf(t, saveFood(t, db, ana, chickenBreast()), saveFood(t, db, ana, kefir()))
	anas.Yield = 1200
	anas = saveRecipe(t, db, ana, anas)
	evasCurry := recipeOf(t, db, eva)
	evas := kitchenDump(t, db, eva)

	join(t, db, ana, ivo)
	evasSoup := copyNamed(t, db, eva, "Soup")
	if members, err := db.RecipeMembers(t.Context(), eva, evasSoup.ID); err != nil || len(members) != 3 {
		t.Errorf("the soup's members = %+v, %v; want Eva's group, the household in it", members, err)
	}
	if members, err := db.RecipeMembers(t.Context(), eva, evasCurry.Food.ID); err != nil || members != nil {
		t.Errorf("Eva's curry's members = %+v, %v; want her alone: Ivo's left for the household's", members, err)
	}
	if got := recipeOf(t, db, eva); !sameDish(got, evasCurry) {
		t.Errorf("Eva's curry = %+v, want it as it was", got)
	}
	if members, err := db.RecipeMembers(t.Context(), ivo, curry.Food.ID); err != nil || len(members) != 2 {
		t.Errorf("Ivo's curry's members = %+v, %v; want the household", members, err)
	}
	if got := recipeOf(t, db, ivo); got.Yield != 1200 || got.Food.ID != curry.Food.ID {
		t.Errorf("Ivo's curry = %+v, want Ana's dish under his id", got)
	}
	if kitchenDump(t, db, eva) != evas {
		t.Errorf("Eva's kitchen changed:\n%s\nwas\n%s", kitchenDump(t, db, eva), evas)
	}
}

// A recipe the invited user left (403) gives way to the household's food
// of its name; one of a group a member of the household is in is theirs
// again, the household's copy.
func TestMergeARecipeLeft(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	eva := addEva(t, db)
	evas := saveRecipe(t, db, eva, curryOf(t, saveFood(t, db, eva, chickenBreast()), saveFood(t, db, eva, kefir())))
	stew := curryOf(t, copyNamed(t, db, eva, "Chicken breast"), copyNamed(t, db, eva, "Kefir"))
	stew.Food.Name, stew.Food.Aliases = "Stew", nil
	stew = saveRecipe(t, db, eva, stew)
	for _, r := range []recipe.Recipe{evas, stew} {
		if err := db.ShareRecipe(t.Context(), eva, testActor, r.Food.ID, ivo, t0); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.ShareRecipe(t.Context(), eva, testActor, stew.Food.ID, ana, t0); err != nil {
		t.Fatal(err)
	}
	ivosCurry, ivosStew := copyNamed(t, db, ivo, "Chicken curry"), copyNamed(t, db, ivo, "Stew")
	for _, r := range []food.Food{ivosCurry, ivosStew} {
		if err := db.UnshareRecipe(t.Context(), ivo, testActor, r.ID, seatOf(t, db, ivo, r.ID, ivo), t0); err != nil {
			t.Fatal(err)
		}
	}
	saveRecipe(t, db, ana, curryOf(t, saveFood(t, db, ana, chickenBreast()), saveFood(t, db, ana, kefir())))

	got := join(t, db, ana, ivo)
	if want := []store.MergeRename{{From: "Chicken curry", To: "Chicken curry (2)"}}; !reflect.DeepEqual(got.Renamed, want) {
		t.Errorf("renamed = %+v, want %+v", got.Renamed, want)
	}
	if !leftBy(t, db, ivo, ivosCurry.ID) {
		t.Errorf("Ivo's curry left is no longer: want it his alone, for his meals")
	}
	if _, err := db.RecipeByID(t.Context(), ivo, ivosStew.ID); err != nil {
		t.Errorf("Ivo's stew: %v, want it back", err)
	}
	if members, err := db.RecipeMembers(t.Context(), eva, stew.Food.ID); err != nil || len(members) != 3 {
		t.Errorf("the stew's members = %+v, %v; want Eva and the household", members, err)
	}
}

// A recipe the two shared before the join is the household's: a save of
// another member's since keeps an undo from taking it back.
func TestMergeARecipeSharedBefore(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	k := kefir()
	anas := saveRecipe(t, db, ana, curryOf(t, saveFood(t, db, ana, chickenBreast()), k), k)
	if err := db.ShareRecipe(t.Context(), ana, testActor, anas.Food.ID, ivo, t0); err != nil {
		t.Fatal(err)
	}
	join(t, db, ana, ivo)

	f := copyNamed(t, db, ana, "Chicken curry")
	f.Barcodes = []string{"2000000000015"}
	saveFood(t, db, ana, f)
	ivos := copyNamed(t, db, ivo, "Chicken curry")
	ivos.Barcodes = nil
	saveFood(t, db, ivo, ivos)
	ivos.Barcodes = []string{"2000000000015"}
	saveFood(t, db, ivo, ivos)
	if _, err := db.UndoLast(t.Context(), ana, testActor, t0.Add(time.Minute)); !isConflict(err, store.UndoChanged) {
		t.Errorf("Ana's undo after Ivo's saves = %v, want it changed since", err)
	}
	if got := copyNamed(t, db, ivo, "Chicken curry"); !slices.Equal(got.Barcodes, []string{"2000000000015"}) {
		t.Errorf("Ivo's curry's barcodes = %v, want his save's", got.Barcodes)
	}
}

// A preview writes nothing; an accept after the household's foods
// changed since its preview is refused, writing nothing, and a new
// preview's accept is done.
func TestAcceptAfterAChangeIsRefused(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	saveFood(t, db, ana, bar())
	saveFood(t, db, ivo, kefir())
	inv := invite(t, db, ana, ivo)
	foods := countRows(t, db, "foods")
	preview, err := db.PreviewHouseholdInvite(t.Context(), ivo, inv.ID, t0)
	if err != nil {
		t.Fatal(err)
	}
	if countRows(t, db, "foods") != foods || countRows(t, db, "household_members") != 0 || countRows(t, db, "food_shares") != 0 {
		t.Fatalf("the preview wrote: %d foods, was %d", countRows(t, db, "foods"), foods)
	}
	again, err := db.PreviewHouseholdInvite(t.Context(), ivo, inv.ID, t0)
	if err != nil || !reflect.DeepEqual(again, preview) {
		t.Errorf("a second preview = %+v, %v; want the first, %+v", again, err, preview)
	}

	corrected := copyNamed(t, db, ana, bar().Name)
	corrected.Nutrients.Kcal = 400
	saveFood(t, db, ana, corrected)
	if _, err := db.AcceptHouseholdInvite(t.Context(), ivo, testActor, inv.ID, preview.Fingerprint, t0); !errors.Is(err, store.ErrMergeChanged) {
		t.Fatalf("the accept after Ana's save = %v, want ErrMergeChanged", err)
	}
	if countRows(t, db, "household_members") != 0 || countRows(t, db, "households") != 0 {
		t.Errorf("a refused accept made a household")
	}
	nothingOf(t, db, ivo, bar().Name)

	preview, err = db.PreviewHouseholdInvite(t.Context(), ivo, inv.ID, t0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AcceptHouseholdInvite(t.Context(), ivo, testActor, inv.ID, preview.Fingerprint, t0); err != nil {
		t.Fatal(err)
	}
	householdHolds(t, db, ana, ivo)
	if got, err := db.HouseholdInvitesTo(t.Context(), ivo, t0); err != nil || len(got) != 0 {
		t.Errorf("Ivo's invites after the accept = %+v, %v; want none", got, err)
	}
}
