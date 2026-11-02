package store_test

import (
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/day"
	"github.com/zarnautovic/tdeemon/internal/domain/food"
	"github.com/zarnautovic/tdeemon/internal/domain/meal"
	"github.com/zarnautovic/tdeemon/internal/domain/recipe"
	"github.com/zarnautovic/tdeemon/internal/store"
)

// sharedCurry is Ana's curry, of her chicken and kefir, shared with Ivo;
// it returns her recipe and Ivo's food of it.
func sharedCurry(t *testing.T, db *store.DB, ana, ivo uuid.UUID) (recipe.Recipe, uuid.UUID) {
	t.Helper()
	chicken := saveFood(t, db, ana, chickenBreast())
	k := kefir()
	r := saveRecipe(t, db, ana, curryOf(t, chicken, k), k)
	if err := db.ShareRecipe(t.Context(), ana, testActor, r.Food.ID, ivo, t0); err != nil {
		t.Fatalf("ShareRecipe: %v", err)
	}
	return r, recipeOf(t, db, ivo).Food.ID
}

// recipeOf is userID's chicken curry, found as a food.
func recipeOf(t *testing.T, db *store.DB, userID uuid.UUID) recipe.Recipe {
	t.Helper()
	f, err := foodNamed(t, db, userID, "Chicken curry")
	if err != nil {
		t.Fatalf("%s's curry: %v", userID, err)
	}
	r, err := db.RecipeByID(t.Context(), userID, f.ID)
	if err != nil {
		t.Fatalf("%s's recipe of the curry: %v", userID, err)
	}
	return r
}

// seatOf is who's seat in the share of viewer's recipe foodID.
func seatOf(t *testing.T, db *store.DB, viewer, foodID, who uuid.UUID) int {
	t.Helper()
	members, err := db.RecipeMembers(t.Context(), viewer, foodID)
	if err != nil {
		t.Fatalf("RecipeMembers: %v", err)
	}
	for _, m := range members {
		if m.ID == who {
			return m.Seat
		}
	}
	t.Fatalf("%v is no member of %v's recipe", who, viewer)
	return 0
}

// sameDish reports whether a and b are one dish: its values, its
// ingredients' count, its weights and portions.
func sameDish(a, b recipe.Recipe) bool {
	return a.Food.Nutrients == b.Food.Nutrients && len(a.Items) == len(b.Items) && a.Yield == b.Yield &&
		a.Portions == b.Portions && a.Pot == b.Pot
}

// undoAt is UndoLast of userID, failing the test on an error.
func undoAt(t *testing.T, db *store.DB, userID uuid.UUID, at time.Duration) {
	t.Helper()
	if _, err := db.UndoLast(t.Context(), userID, testActor, t0.Add(at)); err != nil {
		t.Fatalf("UndoLast: %v", err)
	}
}

// A recipe shared is the recipient's at once, with its values and
// ingredients, of Ana's foods, which are not among Ivo's; none of Ana's
// own words; and the members are listed, the asker first.
func TestShareRecipe(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	r, ivos := sharedCurry(t, db, ana, ivo)
	got := recipeOf(t, db, ivo)
	if got.Food.ID == r.Food.ID || !sameDish(got, r) || len(got.Food.Aliases) != 0 || got.Food.Note != "" {
		t.Errorf("Ivo's curry = %+v, want Ana's dish under his own id, without her words", got)
	}
	for _, it := range got.Items {
		if it.FoodUser != ana {
			t.Errorf("Ivo's ingredient %q is of %v, want Ana's food", it.Name, it.FoodUser)
		}
	}
	if _, err := foodNamed(t, db, ivo, "Chicken breast"); err == nil {
		t.Error("Ana's chicken is among Ivo's foods")
	}
	members, err := db.RecipeMembers(t.Context(), ivo, ivos)
	if err != nil || len(members) != 2 || members[0].ID != ivo || members[1].ID != ana {
		t.Errorf("RecipeMembers(Ivo) = %+v, %v; want Ivo, then Ana", members, err)
	}
}

// A share with a member already, oneself or no user is refused; a recipe
// of none of theirs has no members to list.
func TestShareRefuses(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	r, ivos := sharedCurry(t, db, ana, ivo)
	if err := db.ShareRecipe(t.Context(), ana, testActor, r.Food.ID, ivo, t0); !errors.Is(err, store.ErrShareMember) {
		t.Errorf("a share with a member again = %v, want ErrShareMember", err)
	}
	if err := db.ShareRecipe(t.Context(), ana, testActor, r.Food.ID, ana, t0); !errors.Is(err, store.ErrShareSelf) {
		t.Errorf("a share with oneself = %v, want ErrShareSelf", err)
	}
	if err := db.ShareRecipe(t.Context(), ana, testActor, r.Food.ID, uuid.NewV7(), t0); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("a share with no user = %v, want ErrNotFound", err)
	}
	eva := addEva(t, db)
	if _, err := db.RecipeMembers(t.Context(), eva, ivos); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("RecipeMembers(Eva) = %v, want ErrNotFound", err)
	}
}

// A share to a user with a food of the recipe's name is refused, naming
// none of theirs, and writes nothing.
func TestShareRefusesANameTheRecipientHas(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	chicken := saveFood(t, db, ana, chickenBreast())
	k := kefir()
	r := saveRecipe(t, db, ana, curryOf(t, chicken, k), k)
	theirs := chickenBreast()
	theirs.Name = "Chicken curry"
	saveFood(t, db, ivo, theirs)
	if err := db.ShareRecipe(t.Context(), ana, testActor, r.Food.ID, ivo, t0); !errors.Is(err, store.ErrShareNameTaken) {
		t.Fatalf("ShareRecipe = %v, want ErrShareNameTaken", err)
	}
	if members, err := db.RecipeMembers(t.Context(), ana, r.Food.ID); err != nil || members != nil {
		t.Errorf("RecipeMembers after the refusal = %+v, %v; want none", members, err)
	}
}

// A save by any member is every member's: Ivo's "cooked again" gives Ana
// its weight, and his own food as an ingredient, of his, never among
// hers; each keeps their own words.
func TestSaveOfASharedRecipeIsEveryones(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	_, ivos := sharedCurry(t, db, ana, ivo)
	mine := recipeOf(t, db, ivo)
	rice := food.Food{
		ID: uuid.NewV7(), Name: "Rice", Basis: food.Per100g, Source: food.Reference,
		Nutrients: food.Nutrients{Kcal: 130, Protein: 2.7, Carbs: 28, Fat: 0.3},
	}
	rice = saveFood(t, db, ivo, rice)
	mine.Yield, mine.Food.Note = 1200, "Ivo's words"
	mine.Items = append(mine.Items, itemOf(t, rice, meal.Quantity{Amount: 200, Unit: "g"}))
	saveRecipe(t, db, ivo, mine)

	anas := recipeOf(t, db, ana)
	if anas.Yield != 1200 || len(anas.Items) != 3 || anas.Items[2].FoodUser != ivo || anas.Food.Note != "" {
		t.Errorf("Ana's curry after Ivo's save = %+v, want his weight and rice, of his, and her own note", anas)
	}
	if anas.Food.Aliases[0] != "the curry" {
		t.Errorf("Ana's aliases = %v, want hers", anas.Food.Aliases)
	}
	if _, err := foodNamed(t, db, ana, "Rice"); err == nil {
		t.Error("Ivo's rice is among Ana's foods")
	}
	if got := recipeOf(t, db, ivo); got.Food.ID != ivos || got.Food.Note != "Ivo's words" || got.Items[0].FoodUser != ana {
		t.Errorf("Ivo's curry = %+v, want his, Ana's chicken hers", got)
	}
}

// A save takes every ingredient's food as it is now, a member's
// correction too; a save after another's is whole.
func TestSaveTakesAMembersCorrection(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	_, _ = sharedCurry(t, db, ana, ivo)
	chicken, err := foodNamed(t, db, ana, "Chicken breast")
	if err != nil {
		t.Fatal(err)
	}
	chicken.Nutrients.Kcal = 165
	saveFood(t, db, ana, chicken)
	saveRecipe(t, db, ivo, recipeOf(t, db, ivo))
	for _, u := range []uuid.UUID{ana, ivo} {
		if got := recipeOf(t, db, u); got.Items[0].Nutrients.Kcal != 990 {
			t.Errorf("%v's chicken in the curry = %v kcal, want 990 (600 g of 165)", u, got.Items[0].Nutrients.Kcal)
		}
	}
}

// Two saves in turn: the second is every member's, whole.
func TestSecondSaveOfASharedRecipeWins(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	_, _ = sharedCurry(t, db, ana, ivo)
	first := recipeOf(t, db, ana)
	first.Yield = 900
	saveRecipe(t, db, ana, first)
	second := recipeOf(t, db, ivo)
	second.Yield, second.Portions = 1100, 5
	saveRecipe(t, db, ivo, second)
	for _, u := range []uuid.UUID{ana, ivo} {
		if got := recipeOf(t, db, u); got.Yield != 1100 || got.Portions != 5 {
			t.Errorf("%v's curry = %v g in %d, want 1100 in 5", u, got.Yield, got.Portions)
		}
	}
}

// Another user's food the recipe does not have, of someone in no share,
// is no ingredient of it.
func TestForeignFoodOutsideTheGroupIsRefused(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	_, _ = sharedCurry(t, db, ana, ivo)
	eva := addEva(t, db)
	evas := saveFood(t, db, eva, chickenBreast())
	mine := recipeOf(t, db, ivo)
	it := itemOf(t, evas, meal.Quantity{Amount: 100, Unit: "g"})
	it.FoodUser = eva
	mine.Items = append(mine.Items, it)
	if _, err := db.SaveRecipe(t.Context(), ivo, testActor, mine, nil, t0); !errors.Is(err, store.ErrInvalidRecipe) {
		t.Errorf("a save with Eva's food = %v, want ErrInvalidRecipe", err)
	}
}

// A recipe is not its own ingredient through another member's recipe:
// Ana's curry in Ivo's bowl, and Ivo's bowl in the shared curry.
func TestCycleAcrossMembers(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	_, ivos := sharedCurry(t, db, ana, ivo)
	curry, err := db.FoodByID(t.Context(), ivo, ivos)
	if err != nil {
		t.Fatal(err)
	}
	bowl := saveRecipe(t, db, ivo, recipe.Recipe{
		Food:  food.Food{ID: uuid.NewV7(), Name: "Curry bowl"},
		Items: []meal.Item{itemOf(t, curry, meal.Quantity{Amount: 300, Unit: "g"})},
	})
	mine := recipeOf(t, db, ivo)
	mine.Items = append(mine.Items, itemOf(t, bowl.Food, meal.Quantity{Amount: 100, Unit: "g"}))
	if _, err := db.SaveRecipe(t.Context(), ivo, testActor, mine, nil, t0); !errors.Is(err, store.ErrRecipeCycle) {
		t.Errorf("the bowl in the curry = %v, want ErrRecipeCycle", err)
	}
}

// A member who left keeps their recipe for their meals, but picks it no
// more; the others keep it, and a save writes no more into the leaver's.
func TestSaveSkipsWhoLeft(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	r, ivos := sharedCurry(t, db, ana, ivo)
	curry, err := db.FoodByID(t.Context(), ivo, ivos)
	if err != nil {
		t.Fatal(err)
	}
	it, err := meal.FromFood(curry, meal.Quantity{Amount: 250, Unit: "g"}, "")
	if err != nil {
		t.Fatal(err)
	}
	m := meal.Meal{ID: uuid.NewV7(), EatenAt: t0, Day: day.DateOf(t0), Client: meal.ClientWeb, Items: []meal.Item{it}}
	if _, _, err := db.CreateMeal(t.Context(), ivo, testActor, m, t0); err != nil {
		t.Fatal(err)
	}
	if err := db.UnshareRecipe(t.Context(), ivo, testActor, ivos, seatOf(t, db, ivo, ivos, ivo), t0); err != nil {
		t.Fatalf("Ivo leaves: %v", err)
	}
	if _, err := db.FoodByID(t.Context(), ivo, ivos); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Ivo's FoodByID of the curry he left = %v, want ErrNotFound", err)
	}
	if found, _ := db.FindFoods(t.Context(), ivo, "curry", 10); len(found) != 0 {
		t.Errorf("Ivo's search of curry = %v, want none", found)
	}
	if got, err := db.MealByID(t.Context(), ivo, m.ID); err != nil || got.Items[0].Nutrients.Kcal != it.Nutrients.Kcal {
		t.Errorf("Ivo's meal of it = %+v, %v; want it as logged", got, err)
	}
	// Ana alone has it: an ordinary recipe again, which she changes.
	if members, err := db.RecipeMembers(t.Context(), ana, r.Food.ID); err != nil || members != nil {
		t.Errorf("Ana's members after Ivo left = %+v, %v; want none", members, err)
	}
	anas := recipeOf(t, db, ana)
	anas.Yield = 800
	saveRecipe(t, db, ana, anas)
	var yield float64
	if err := db.Read().QueryRowContext(t.Context(), "SELECT yield_g FROM recipes WHERE user_id = ? AND food_id = ?",
		ivo.String(), ivos.String()).Scan(&yield); err != nil || yield != 1000 {
		t.Errorf("Ivo's recipe after Ana's save = %v g, %v; want it as he left it", yield, err)
	}
}

// A member shares on, and a recipe shared again with one who left gives
// them their recipe back, which their meals name still.
func TestShareOnAndAgain(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	r, ivos := sharedCurry(t, db, ana, ivo)
	eva := addEva(t, db)
	if err := db.ShareRecipe(t.Context(), ivo, testActor, ivos, eva, t0); err != nil {
		t.Fatalf("Ivo shares with Eva: %v", err)
	}
	if members, err := db.RecipeMembers(t.Context(), ana, r.Food.ID); err != nil || len(members) != 3 {
		t.Errorf("members after Ivo shared on = %+v, %v; want three", members, err)
	}
	evas := recipeOf(t, db, eva).Food.ID
	if err := db.UnshareRecipe(t.Context(), eva, testActor, evas, seatOf(t, db, eva, evas, ivo), t0); err != nil {
		t.Fatalf("Eva stops Ivo: %v", err)
	}
	if err := db.ShareRecipe(t.Context(), ana, testActor, r.Food.ID, ivo, t0); err != nil {
		t.Fatalf("Ana shares with Ivo again: %v", err)
	}
	if got := recipeOf(t, db, ivo); got.Food.ID != ivos {
		t.Errorf("Ivo's curry again = %v, want his old one, %v", got.Food.ID, ivos)
	}
}

// A member's account deleted, the others keep every ingredient, of a food
// of their own now, and save the recipe.
func TestDeletedMembersIngredientsStay(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	_, _ = sharedCurry(t, db, ana, ivo)
	if _, err := db.DeleteUser(t.Context(), ana); err != nil {
		t.Fatal(err)
	}
	mine := recipeOf(t, db, ivo)
	if len(mine.Items) != 2 || mine.Items[0].FoodUser != uuid.Nil() {
		t.Fatalf("Ivo's curry after Ana went = %+v, want her ingredients, his", mine.Items)
	}
	chicken, err := foodNamed(t, db, ivo, "Chicken breast")
	if err != nil || chicken.ID != mine.Items[0].FoodID || chicken.Nutrients.Kcal != 110 {
		t.Errorf("Ivo's chicken = %+v, %v; want Ana's, his now", chicken, err)
	}
	saveRecipe(t, db, ivo, mine)
}

// An undo of a share takes the recipe from the member it gave it to,
// alone: those they shared it with keep it.
func TestUndoOfAShareTakesItFromThatMemberAlone(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	r, ivos := sharedCurry(t, db, ana, ivo)
	eva := addEva(t, db)
	if err := db.ShareRecipe(t.Context(), ivo, testActor, ivos, eva, t0); err != nil {
		t.Fatal(err)
	}
	// Ana's share is no longer her latest change of it: Ivo's share
	// moved the group on, so her undo passes it over.
	if _, err := db.UndoLast(t.Context(), ana, testActor, t0.Add(time.Minute)); err == nil {
		if members, _ := db.RecipeMembers(t.Context(), ana, r.Food.ID); len(members) != 3 {
			t.Errorf("Ana's undo after Ivo's share changed the members: %+v", members)
		}
	}
	undoAt(t, db, ivo, time.Minute)
	if members, err := db.RecipeMembers(t.Context(), ana, r.Food.ID); err != nil || len(members) != 2 {
		t.Errorf("members after Ivo's undo = %+v, %v; want Ana and Ivo", members, err)
	}
	if _, err := foodNamed(t, db, eva, "Chicken curry"); err == nil {
		t.Error("Eva keeps a curry after the share was undone")
	}
}

// An undo of a save after another member's is refused; one of the
// latest is every member's.
func TestUndoOfASharedSave(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	_, _ = sharedCurry(t, db, ana, ivo)
	anas := recipeOf(t, db, ana)
	anas.Yield = 900
	saveRecipe(t, db, ana, anas)
	undoAt(t, db, ana, time.Minute)
	if got := recipeOf(t, db, ivo); got.Yield != 1000 {
		t.Errorf("Ivo's curry after Ana undid her save = %v g, want 1000", got.Yield)
	}

	anas = recipeOf(t, db, ana)
	anas.Yield = 950
	saveRecipe(t, db, ana, anas)
	ivos := recipeOf(t, db, ivo)
	ivos.Yield = 1100
	saveRecipe(t, db, ivo, ivos)
	var conflict *store.UndoConflictError
	if _, err := db.UndoLast(t.Context(), ana, testActor, t0.Add(2*time.Minute)); !errors.As(err, &conflict) {
		t.Errorf("Ana's undo after Ivo's save = %v, want a conflict", err)
	}
	if got := recipeOf(t, db, ana); got.Yield != 1100 {
		t.Errorf("Ana's curry after her refused undo = %v g, want Ivo's 1100", got.Yield)
	}
}

// The creation of a shared recipe is not undone while it is shared; a
// leave is, giving the recipe back.
func TestUndoOfCreationAndOfALeave(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	r, ivos := sharedCurry(t, db, ana, ivo)
	if err := db.UnshareRecipe(t.Context(), ivo, testActor, ivos, seatOf(t, db, ivo, ivos, ivo), t0); err != nil {
		t.Fatal(err)
	}
	undoAt(t, db, ivo, time.Minute)
	if members, err := db.RecipeMembers(t.Context(), ana, r.Food.ID); err != nil || len(members) != 2 {
		t.Errorf("members after Ivo undid his leave = %+v, %v; want both", members, err)
	}
	if _, err := db.FoodByID(t.Context(), ivo, ivos); err != nil {
		t.Errorf("Ivo's curry after the undo of his leave: %v", err)
	}
	// Ana's latest is the share, then the save that made the curry.
	var conflict *store.UndoConflictError
	db2, a2, i2 := withTwoUsers(t)
	chicken := saveFood(t, db2, a2, chickenBreast())
	k := kefir()
	made := saveRecipe(t, db2, a2, curryOf(t, chicken, k), k)
	if err := db2.ShareRecipe(t.Context(), i2, testActor, made.Food.ID, a2, t0); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Ivo shares Ana's recipe = %v, want ErrNotFound", err)
	}
	if err := db2.ShareRecipe(t.Context(), a2, testActor, made.Food.ID, i2, t0); err != nil {
		t.Fatal(err)
	}
	i2s := recipeOf(t, db2, i2).Food.ID
	if err := db2.UnshareRecipe(t.Context(), i2, testActor, i2s, seatOf(t, db2, i2, i2s, a2), t0); err != nil {
		t.Fatal(err)
	}
	if err := db2.ShareRecipe(t.Context(), i2, testActor, recipeOf(t, db2, i2).Food.ID, a2, t0); err != nil {
		t.Fatal(err)
	}
	// Ana's latest is the share she made, undone by the group moving on:
	// passed over; then the creation, refused while shared.
	for range 2 {
		if _, err := db2.UndoLast(t.Context(), a2, testActor, t0.Add(time.Minute)); !errors.As(err, &conflict) {
			t.Errorf("Ana's undo = %v, want a conflict", err)
		}
	}
	if _, err := db2.RecipeByID(t.Context(), i2, recipeOf(t, db2, i2).Food.ID); err != nil {
		t.Errorf("Ivo's curry after Ana's undos: %v", err)
	}
}

// An ingredient of a member who left keeps the values the recipe has: a
// change of their food after it stays theirs. While they are a member,
// IngredientFoods gives their food; then none.
func TestALeaversIngredientKeepsItsValues(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	r, ivos := sharedCurry(t, db, ana, ivo)
	if foods, err := db.IngredientFoods(t.Context(), ivo, ivos); err != nil || len(foods) != 2 {
		t.Fatalf("Ivo's IngredientFoods = %v, %v; want Ana's two", foods, err)
	}
	if err := db.UnshareRecipe(t.Context(), ana, testActor, r.Food.ID, seatOf(t, db, ana, r.Food.ID, ana), t0); err != nil {
		t.Fatal(err)
	}
	chicken, err := foodNamed(t, db, ana, "Chicken breast")
	if err != nil {
		t.Fatal(err)
	}
	chicken.Name, chicken.Nutrients.Kcal = "Ana's secret chicken", 300
	saveFood(t, db, ana, chicken)
	saveRecipe(t, db, ivo, recipeOf(t, db, ivo))
	got := recipeOf(t, db, ivo)
	if got.Items[0].Nutrients.Kcal != 660 || got.Items[0].Name != "Chicken breast" {
		t.Errorf("Ivo's chicken after Ana left and changed hers = %+v, want it as the recipe had it", got.Items[0])
	}
	if foods, err := db.IngredientFoods(t.Context(), ivo, ivos); err != nil || len(foods) != 0 {
		t.Errorf("Ivo's IngredientFoods after Ana left = %v, %v; want none", foods, err)
	}
}

// A save that brings back an earlier value after another member's is
// still another change: Ana's undo of hers is refused, and leaves
// Ivo's.
func TestUndoAfterAnothersSavesBackIsRefused(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	_, _ = sharedCurry(t, db, ana, ivo)
	for _, s := range []struct {
		who   uuid.UUID
		yield float64
	}{{ana, 900}, {ivo, 1100}, {ivo, 900}} {
		r := recipeOf(t, db, s.who)
		r.Yield = s.yield
		saveRecipe(t, db, s.who, r)
	}
	var conflict *store.UndoConflictError
	if _, err := db.UndoLast(t.Context(), ana, testActor, t0.Add(time.Minute)); !errors.As(err, &conflict) {
		t.Errorf("Ana's undo = %v, want a conflict", err)
	}
	if got := recipeOf(t, db, ivo); got.Yield != 900 {
		t.Errorf("Ivo's curry = %v g, want his 900", got.Yield)
	}
}

// A deleted member's ingredients are copies of their own for the others,
// under a name of their own when they have it, and a recipe among them a
// plain food: never a food of theirs that may be made of the recipe.
func TestAdoptionMakesCopies(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	mine := chickenBreast()
	mine.Nutrients.Kcal = 120
	saveFood(t, db, ivo, mine)
	_, _ = sharedCurry(t, db, ana, ivo)
	if _, err := db.DeleteUser(t.Context(), ana); err != nil {
		t.Fatal(err)
	}
	got := recipeOf(t, db, ivo)
	adopted, err := db.FoodByID(t.Context(), ivo, got.Items[0].FoodID)
	if err != nil || adopted.Name != "Chicken breast (2)" || adopted.Nutrients.Kcal != 110 {
		t.Errorf("Ivo's adopted chicken = %+v, %v; want Ana's, as Chicken breast (2)", adopted, err)
	}
	if had, err := foodNamed(t, db, ivo, "Chicken breast"); err != nil || had.Nutrients.Kcal != 120 {
		t.Errorf("Ivo's own chicken = %+v, %v; want it as it was", had, err)
	}
}

// A recipe the user left is found by no name: a new food of its name is
// refused, as a pick of it would be; and its meals repeat no more.
func TestALeftRecipeIsNoPickByName(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	_, ivos := sharedCurry(t, db, ana, ivo)
	curry, err := db.FoodByID(t.Context(), ivo, ivos)
	if err != nil {
		t.Fatal(err)
	}
	it, err := meal.FromFood(curry, meal.Quantity{Amount: 250, Unit: "g"}, "")
	if err != nil {
		t.Fatal(err)
	}
	m := meal.Meal{ID: uuid.NewV7(), EatenAt: t0, Day: day.DateOf(t0), Client: meal.ClientWeb, Items: []meal.Item{it}}
	if _, _, err := db.CreateMeal(t.Context(), ivo, testActor, m, t0); err != nil {
		t.Fatal(err)
	}
	if err := db.UnshareRecipe(t.Context(), ivo, testActor, ivos, seatOf(t, db, ivo, ivos, ivo), t0); err != nil {
		t.Fatal(err)
	}
	if _, err := db.FoodByKey(t.Context(), ivo, "Chicken curry", ""); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("FoodByKey of the curry Ivo left = %v, want ErrNotFound", err)
	}
	again := food.Food{
		ID: uuid.NewV7(), Name: "Chicken curry", Basis: food.Per100g, Source: food.Estimate,
		Nutrients: food.Nutrients{Kcal: 120, Protein: 10, Carbs: 5, Fat: 6},
	}
	item := meal.Item{
		FoodID: again.ID, Quantity: meal.Quantity{Amount: 200, Unit: "g"}, Name: again.Name, Amount: 200, Unit: "g",
		Nutrients: food.Nutrients{Kcal: 240, Protein: 20, Carbs: 10, Fat: 12}, Source: food.Estimate,
	}
	m2 := meal.Meal{ID: uuid.NewV7(), EatenAt: t0, Day: day.DateOf(t0), Client: meal.ClientWeb, Items: []meal.Item{item}}
	if _, _, err := db.CreateMealWithFoods(t.Context(), ivo, testActor, m2, []food.Food{again}, t0); !errors.Is(err, store.ErrNameTaken) {
		t.Errorf("a meal of a new food of the curry's name = %v, want ErrNameTaken", err)
	}
	if recent, err := db.RecentMeals(t.Context(), ivo, 10); err != nil || len(recent) != 0 {
		t.Errorf("Ivo's recent meals = %v, %v; want none of the curry he left", recent, err)
	}
}

// A seat is never one of a member before: one whose account went leaves
// their seat empty.
func TestSeatsAreNotReused(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	r, ivos := sharedCurry(t, db, ana, ivo)
	ivosSeat := seatOf(t, db, ivo, ivos, ivo)
	if _, err := db.DeleteUser(t.Context(), ivo); err != nil {
		t.Fatal(err)
	}
	eva := addEva(t, db)
	if err := db.ShareRecipe(t.Context(), ana, testActor, r.Food.ID, eva, t0); err != nil {
		t.Fatal(err)
	}
	if seat := seatOf(t, db, ana, r.Food.ID, eva); seat == ivosSeat {
		t.Errorf("Eva's seat = %d, Ivo's before", seat)
	}
}

// An ingredient of another member's food stays theirs in a save, though
// the saver has a food of the same id.
func TestAMembersIngredientBeforeOnesOwn(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	_, _ = sharedCurry(t, db, ana, ivo)
	got := recipeOf(t, db, ivo)
	mine := chickenBreast()
	mine.ID, mine.Name, mine.Nutrients.Kcal = got.Items[0].FoodID, "Ivo's twin", 300
	saveFood(t, db, ivo, mine)
	saveRecipe(t, db, ivo, got)
	if after := recipeOf(t, db, ivo); after.Items[0].FoodUser != ana || after.Items[0].Nutrients.Kcal != 660 {
		t.Errorf("Ivo's chicken after his save = %+v, want Ana's", after.Items[0])
	}
}

// A frozen ingredient given in servings still reads: a save of the
// recipe after its owner left takes it as it was.
func TestAFrozenIngredientInServings(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	egg := food.Food{
		ID: uuid.NewV7(), Name: "Egg", Basis: food.Per100g, Source: food.Reference,
		Nutrients: food.Nutrients{Kcal: 143, Protein: 12.6, Carbs: 0.7, Fat: 9.5}, Servings: []food.Serving{{Name: "piece", Amount: 60}},
	}
	egg = saveFood(t, db, ana, egg)
	omelette := saveRecipe(t, db, ana, recipe.Recipe{
		Food:  food.Food{ID: uuid.NewV7(), Name: "Chicken curry"},
		Items: []meal.Item{itemOf(t, egg, meal.Quantity{Servings: 2, Serving: "piece"})},
	})
	if err := db.ShareRecipe(t.Context(), ana, testActor, omelette.Food.ID, ivo, t0); err != nil {
		t.Fatal(err)
	}
	if err := db.UnshareRecipe(t.Context(), ana, testActor, omelette.Food.ID, seatOf(t, db, ana, omelette.Food.ID, ana), t0); err != nil {
		t.Fatal(err)
	}
	saveRecipe(t, db, ivo, recipeOf(t, db, ivo))
	if got := recipeOf(t, db, ivo); got.Items[0].Amount != 120 || got.Items[0].Nutrients.Kcal != 171.6 {
		t.Errorf("Ivo's eggs = %+v, want two pieces, 120 g", got.Items[0])
	}
}

// In a shared recipe, an archived food of one's own keeps the values the
// recipe has, as another member's does: every member's save takes it
// alike.
func TestAnArchivedIngredientKeepsItsValues(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	_, _ = sharedCurry(t, db, ana, ivo)
	chicken, err := foodNamed(t, db, ana, "Chicken breast")
	if err != nil {
		t.Fatal(err)
	}
	chicken.Nutrients.Kcal = 165
	chicken = saveFood(t, db, ana, chicken)
	if err := db.SetFoodArchived(t.Context(), ana, testActor, chicken.ID, true, t0); err != nil {
		t.Fatal(err)
	}
	saveRecipe(t, db, ana, recipeOf(t, db, ana))
	if got := recipeOf(t, db, ana); got.Items[0].Nutrients.Kcal != 660 {
		t.Errorf("Ana's chicken = %v kcal, want the 660 the recipe had", got.Items[0].Nutrients.Kcal)
	}
}

// A deleted member's ingredient of a member who had left is adopted as
// the recipe keeps it, not as their food came to be since.
func TestAdoptionOfAFrozenIngredient(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	r, _ := sharedCurry(t, db, ana, ivo)
	if err := db.UnshareRecipe(t.Context(), ana, testActor, r.Food.ID, seatOf(t, db, ana, r.Food.ID, ana), t0); err != nil {
		t.Fatal(err)
	}
	chicken, err := foodNamed(t, db, ana, "Chicken breast")
	if err != nil {
		t.Fatal(err)
	}
	chicken.Name, chicken.Nutrients.Kcal = "Ana's secret chicken", 300
	saveFood(t, db, ana, chicken)
	if _, err := db.DeleteUser(t.Context(), ana); err != nil {
		t.Fatal(err)
	}
	got := recipeOf(t, db, ivo)
	adopted, err := db.FoodByID(t.Context(), ivo, got.Items[0].FoodID)
	if err != nil || adopted.Name != "Chicken breast" || adopted.Nutrients.Kcal != 110 {
		t.Errorf("Ivo's adopted chicken = %+v, %v; want it as the recipe kept it", adopted, err)
	}
}

// Two undos of one's own saves in turn put a shared recipe back twice.
func TestUndoTwiceOfASharedRecipe(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	_, _ = sharedCurry(t, db, ana, ivo)
	for _, yield := range []float64{900, 800} {
		r := recipeOf(t, db, ana)
		r.Yield = yield
		saveRecipe(t, db, ana, r)
	}
	undoAt(t, db, ana, time.Minute)
	undoAt(t, db, ana, 2*time.Minute)
	if got := recipeOf(t, db, ivo); got.Yield != 1000 {
		t.Errorf("Ivo's curry after Ana's two undos = %v g, want 1000", got.Yield)
	}
}

// A recipe the user left is theirs to save no more, unchanged too.
func TestALeftRecipeIsNotSaved(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	_, ivos := sharedCurry(t, db, ana, ivo)
	mine := recipeOf(t, db, ivo)
	if err := db.UnshareRecipe(t.Context(), ivo, testActor, ivos, seatOf(t, db, ivo, ivos, ivo), t0); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CorrectRecipe(t.Context(), ivo, testActor, mine, nil, t0); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Ivo's correction of the curry he left = %v, want ErrNotFound", err)
	}
}

// An adopted food's name keeps to the length a name may have.
func TestAnAdoptedNameKeepsItsLength(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	long := strings.Repeat("x", food.MaxNameLen)
	mine := chickenBreast()
	mine.Name = long
	saveFood(t, db, ivo, mine)
	hers := chickenBreast()
	hers.Name = long
	hers = saveFood(t, db, ana, hers)
	r := saveRecipe(t, db, ana, recipe.Recipe{
		Food:  food.Food{ID: uuid.NewV7(), Name: "Chicken curry"},
		Items: []meal.Item{itemOf(t, hers, meal.Quantity{Amount: 100, Unit: "g"})},
	})
	if err := db.ShareRecipe(t.Context(), ana, testActor, r.Food.ID, ivo, t0); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DeleteUser(t.Context(), ana); err != nil {
		t.Fatal(err)
	}
	adopted, err := db.FoodByID(t.Context(), ivo, recipeOf(t, db, ivo).Items[0].FoodID)
	if err != nil || len([]rune(adopted.Name)) > food.MaxNameLen || adopted.Validate() != nil {
		t.Errorf("Ivo's adopted food = %q (%d), %v", adopted.Name, len([]rune(adopted.Name)), err)
	}
}
