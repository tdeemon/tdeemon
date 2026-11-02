package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/event"
	"github.com/zarnautovic/tdeemon/internal/domain/food"
	"github.com/zarnautovic/tdeemon/internal/domain/meal"
	"github.com/zarnautovic/tdeemon/internal/domain/recipe"
)

// Why a recipe, or a food of one, cannot be saved as asked.
var (
	// ErrInvalidRecipe: the recipe breaks a rule of package recipe, as
	// its ingredients came out once their new foods were saved.
	ErrInvalidRecipe = errors.New("invalid recipe")
	// ErrRecipeCycle: an ingredient is the recipe, through the recipes
	// it is made of.
	ErrRecipeCycle = errors.New("the recipe is an ingredient of one of its ingredients")
	// ErrRecipeFood: a save of a food would change a recipe's food, whose
	// values change only with its recipe (SaveRecipe), or make a food of
	// source recipe that has no recipe.
	ErrRecipeFood = errors.New("a recipe's food changes only with its recipe")
)

// SaveRecipe stores r, a recipe of userID's, worked out of its
// ingredients (recipe.Make), and returns it as stored. As SaveFood does
// for a food, it corrects the recipe of r's id, or else the one of r's
// name, or adds it; a food of the name that is no recipe is in the way, a
// *FoodConflictError of ErrNameTaken, and so is a food of an alias of
// r's (ErrAliasTaken). foods are new foods its ingredients name by their
// ids, saved first as a meal's are (CreateMealWithFoods): a food of the
// name and brand of one the user has is that food, and the ingredient is
// worked out of it. The recipe is saved whole: its food and ingredients
// replace the ones before, each ingredient with the values its food
// brings now. It is recorded as a's event, with the recipe before (none
// for a new one) and after, the new foods' writes parts of it.
func (db *DB) SaveRecipe(ctx context.Context, userID uuid.UUID, a event.Actor, r recipe.Recipe, foods []food.Food, now time.Time) (recipe.Recipe, error) {
	return db.saveRecipe(ctx, userID, a, r, foods, saveOrCorrect, now)
}

// CreateRecipe is SaveRecipe of a new recipe only: a food of its name,
// archived too, is in the way, a *FoodConflictError of ErrNameTaken.
func (db *DB) CreateRecipe(ctx context.Context, userID uuid.UUID, a event.Actor, r recipe.Recipe, foods []food.Food, now time.Time) (recipe.Recipe, error) {
	return db.saveRecipe(ctx, userID, a, r, foods, createOnly, now)
}

// CorrectRecipe is SaveRecipe of userID's recipe of r's id, which must
// be one of theirs: ErrNotFound otherwise.
func (db *DB) CorrectRecipe(ctx context.Context, userID uuid.UUID, a event.Actor, r recipe.Recipe, foods []food.Food, now time.Time) (recipe.Recipe, error) {
	return db.saveRecipe(ctx, userID, a, r, foods, correctOnly, now)
}

func (db *DB) saveRecipe(ctx context.Context, userID uuid.UUID, a event.Actor, r recipe.Recipe, foods []food.Food, mode saveMode,
	now time.Time,
) (recipe.Recipe, error) {
	if err := r.Validate(); err != nil {
		return recipe.Recipe{}, fmt.Errorf("%w: %w", ErrInvalidRecipe, err)
	}
	if err := a.Validate(); err != nil {
		return recipe.Recipe{}, fmt.Errorf("invalid actor: %w", err)
	}
	if err := validFoods(foods); err != nil {
		return recipe.Recipe{}, err
	}
	var stored recipe.Recipe
	err := db.WriteTx(ctx, func(tx *sql.Tx) error {
		var err error
		stored, err = saveRecipeInTx(ctx, tx, userID, a, r, foods, mode, now)
		return err
	})
	switch {
	case errors.Is(err, ErrNotFound) || refusedFood(err) || errors.Is(err, ErrInvalidRecipe) || errors.Is(err, ErrRecipeCycle):
		return recipe.Recipe{}, err
	case err != nil:
		if _, ok := errors.AsType[*FoodItemError](err); ok {
			return recipe.Recipe{}, err
		}
		return recipe.Recipe{}, fmt.Errorf("save recipe: %w", err)
	}
	return stored, nil
}

func saveRecipeInTx(ctx context.Context, tx *sql.Tx, userID uuid.UUID, a event.Actor, r recipe.Recipe, foods []food.Food, mode saveMode,
	now time.Time,
) (recipe.Recipe, error) {
	nameKey := food.Key(r.Food.Name)
	target, exists, err := saveTarget(ctx, tx, userID, r.Food.ID, nameKey, "", mode)
	if err != nil {
		return recipe.Recipe{}, err
	}
	if err := checkNotLeft(ctx, tx, userID, r.Food.ID, target, exists); err != nil {
		return recipe.Recipe{}, err
	}
	var before *recipe.Recipe
	if exists {
		had, err := recipeBefore(ctx, tx, userID, target, mode)
		if err != nil {
			return recipe.Recipe{}, err
		}
		before = &had
		// A recipe's food keeps the barcodes a save of the recipe does not
		// name: they are its food's, added as any food's are.
		if r.Food.Barcodes == nil {
			r.Food.Barcodes = had.Food.Barcodes
		}
	}
	var parts []uuid.UUID
	if r.Items, parts, err = ingredients(ctx, tx, userID, a, target, r.Items, foods, now); err != nil {
		return recipe.Recipe{}, err
	}
	r.Food.ID = target
	// A new ingredient may have taken the recipe's name.
	if err := checkNameFree(ctx, tx, userID, target, nameKey); err != nil {
		return recipe.Recipe{}, err
	}
	if err := checkCycle(ctx, tx, userID, target, r.Items); err != nil {
		return recipe.Recipe{}, err
	}
	made, err := recipe.Make(r)
	if err != nil {
		return recipe.Recipe{}, fmt.Errorf("%w: %w", ErrInvalidRecipe, err)
	}
	if err := errors.Join(checkAliases(ctx, tx, userID, made.Food), checkBarcodes(ctx, tx, userID, made.Food)); err != nil {
		return recipe.Recipe{}, err
	}
	if exists {
		err = updateFood(ctx, tx, userID, made.Food, nameKey, "", now)
	} else {
		err = insertFood(ctx, tx, userID, made.Food, nameKey, "", now)
	}
	if err != nil {
		return recipe.Recipe{}, err
	}
	if err := writeRecipe(ctx, tx, userID, made); err != nil {
		return recipe.Recipe{}, err
	}
	return recordRecipe(ctx, tx, userID, a, target, before, parts, now)
}

// checkNotLeft refuses a save of userID's recipe target, when it is one
// they left: theirs to change no more, none to correct by its id (sent),
// and its name in the way.
func checkNotLeft(ctx context.Context, tx *sql.Tx, userID, sent, target uuid.UUID, exists bool) error {
	left, err := withdrawn(ctx, tx, userID, target)
	switch {
	case err != nil || !exists || !left:
		return err
	case sent == target:
		return ErrNotFound
	}
	return conflict(ctx, tx, userID, ErrNameTaken, target.String(), "")
}

// recordRecipe reads userID's recipe id as saved, writes it into its
// share's other members' recipes, and records the save, its new foods'
// writes parts of it.
func recordRecipe(ctx context.Context, tx *sql.Tx, userID uuid.UUID, a event.Actor, id uuid.UUID, before *recipe.Recipe, parts []uuid.UUID,
	now time.Time,
) (recipe.Recipe, error) {
	stored, err := recipeInTx(ctx, tx, userID, id)
	if err != nil {
		return recipe.Recipe{}, err
	}
	// A save that changes nothing (a retry) records nothing, and writes
	// nothing into the share.
	if before != nil && len(parts) == 0 && reflect.DeepEqual(*before, stored) {
		return stored, nil
	}
	share, version, err := shareOf(ctx, tx, userID, id)
	if err != nil {
		return recipe.Recipe{}, err
	}
	was := recipeState{ShareVersion: version}
	if before != nil {
		was.Recipe = *before
	}
	// A shared recipe's save is every member's.
	if err := syncShared(ctx, tx, userID, id, stored, now); err != nil {
		return recipe.Recipe{}, err
	}
	is := recipeState{Recipe: stored}
	if share != "" {
		if is.ShareVersion, err = shareVersion(ctx, tx, share); err != nil {
			return recipe.Recipe{}, err
		}
	}
	var wasState any = was
	if before == nil {
		wasState = nil
	}
	// A save that changes the recipe not, but its new ingredients (an
	// archived food brought back), is a change all the same, which undo
	// takes back with them.
	saved, err := recordEvent(ctx, tx, userID, a, event.RecipeSave, id.String(), wasState, is, len(parts) > 0, now)
	if err != nil {
		return recipe.Recipe{}, err
	}
	return stored, markParts(ctx, tx, userID, saved, parts)
}

// recipeState is a recipe as the events of its saves record it, with its
// share's version (none for a recipe shared with no one): an undo of a
// save after another member's, which moved the version on, finds the
// recipe changed since though its values came back the same.
type recipeState struct {
	recipe.Recipe
	ShareVersion int64 `json:"share_version,omitempty"`
}

// ingredients saves the new foods of foods that items name, as a meal's
// are saved, and returns items of the foods they are stored under, each
// worked out of its food as it is now (freshItems), with the events of
// the foods' writes. recipeID is the recipe they are of.
func ingredients(ctx context.Context, tx *sql.Tx, userID uuid.UUID, a event.Actor, recipeID uuid.UUID, items []meal.Item, foods []food.Food,
	now time.Time,
) ([]meal.Item, []uuid.UUID, error) {
	brought, err := saveNewFoods(ctx, tx, userID, a, foods, items, now)
	if err != nil {
		return nil, nil, err
	}
	if items, err = itemsOf(items, brought.ids, brought.kept); err != nil {
		return nil, nil, err
	}
	items, err = freshItems(ctx, tx, userID, recipeID, items)
	return items, brought.parts, err
}

// freshItems returns items of userID's recipe recipeID worked out of
// their foods as they are now, in the quantity each was given: a recipe
// saved takes its ingredients' values as they are then, whatever values
// the client sent. An ingredient's food is userID's, or, in a shared
// recipe, another member's that the recipe has already
// (ingredientSource), read as that member has it, or kept as the recipe
// has it when that member left or archived it. An ingredient of no such food, or in a
// quantity its food no longer takes (a serving taken away), is
// ErrInvalidRecipe.
func freshItems(ctx context.Context, tx *sql.Tx, userID, recipeID uuid.UUID, items []meal.Item) ([]meal.Item, error) {
	share, _, err := shareOf(ctx, tx, userID, recipeID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	stored, err := storedItems(ctx, tx, userID, recipeID)
	if err != nil {
		return nil, err
	}
	out := make([]meal.Item, len(items))
	for i := range items {
		f, owner, _, err := ingredientSource(ctx, tx, userID, share, stored, &items[i])
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("%w: ingredient %q: no such food", ErrInvalidRecipe, items[i].Name)
		}
		if err != nil {
			return nil, err
		}
		if out[i], err = meal.FromFood(f, items[i].Quantity, items[i].Note); err != nil {
			return nil, fmt.Errorf("%w: ingredient %w", ErrInvalidRecipe, err)
		}
		out[i].FoodUser = owner
	}
	return out, nil
}

// recipeBefore returns userID's recipe id, which a save in mode writes
// to: a food of its id that is no recipe is none to correct
// (ErrNotFound), and one of its name is in the way.
func recipeBefore(ctx context.Context, tx *sql.Tx, userID, id uuid.UUID, mode saveMode) (recipe.Recipe, error) {
	had, err := recipeInTx(ctx, tx, userID, id)
	switch {
	case errors.Is(err, ErrNotFound) && mode == correctOnly:
		return recipe.Recipe{}, ErrNotFound
	case errors.Is(err, ErrNotFound):
		return recipe.Recipe{}, conflict(ctx, tx, userID, ErrNameTaken, id.String(), "")
	}
	return had, err
}

// checkNameFree refuses the name of name key for userID's recipe id when
// another food has it, without a brand, as a recipe has none.
func checkNameFree(ctx context.Context, tx *sql.Tx, userID, id uuid.UUID, nameKey string) error {
	var other string
	err := tx.QueryRowContext(ctx, "SELECT id FROM foods WHERE user_id = ? AND name_key = ? AND brand_key = '' AND id <> ?",
		userID.String(), nameKey, id.String()).Scan(&other)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return conflict(ctx, tx, userID, ErrNameTaken, other, "")
}

// checkCycle refuses items of userID's recipe id when one of them is the
// recipe, or a recipe made of it, at any depth (ErrRecipeCycle): across
// users, through shared recipes, where any member's recipe of its share
// is the recipe.
func checkCycle(ctx context.Context, tx *sql.Tx, userID, id uuid.UUID, items []meal.Item) error {
	share, _, err := shareOf(ctx, tx, userID, id)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	for i := range items {
		if items[i].FoodID == id && items[i].FoodUser == uuid.Nil() {
			return fmt.Errorf("%w: %q", ErrRecipeCycle, items[i].Name)
		}
		owner := items[i].FoodUser
		if owner == uuid.Nil() {
			owner = userID
		}
		var one int
		err := tx.QueryRowContext(ctx, `WITH RECURSIVE reach (uid, id) AS (
				SELECT ?1, ?2
				UNION
				SELECT ri.food_user_id, ri.food_id FROM recipe_items AS ri
					JOIN reach ON ri.user_id = reach.uid AND ri.recipe_id = reach.id
				WHERE ri.food_id IS NOT NULL
			)
			SELECT 1 FROM reach
			WHERE (reach.uid = ?3 AND reach.id = ?4)
				OR EXISTS (SELECT 1 FROM recipe_shares AS rs
					WHERE rs.share_id = ?5 AND rs.user_id = reach.uid AND rs.food_id = reach.id)
			LIMIT 1`,
			owner.String(), items[i].FoodID.String(), userID.String(), id.String(), share).Scan(&one)
		if err == nil {
			return fmt.Errorf("%w: %q", ErrRecipeCycle, items[i].Name)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	return nil
}

// writeRecipe writes r's food's servings, aliases and search row, and
// its recipe row and ingredients, in place of the ones before; its foods
// row is written.
func writeRecipe(ctx context.Context, tx *sql.Tx, userID uuid.UUID, r recipe.Recipe) error {
	if err := replaceFoodChildren(ctx, tx, userID, r.Food); err != nil {
		return err
	}
	id, uid := r.Food.ID.String(), userID.String()
	// In place, not deleted and made anew: a share names the recipes row.
	if _, err := tx.ExecContext(ctx, `INSERT INTO recipes (food_id, user_id, yield_g, portions, pot_g) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (user_id, food_id) DO UPDATE SET yield_g = excluded.yield_g, portions = excluded.portions, pot_g = excluded.pot_g`,
		id, uid, positive(r.Yield), positive(float64(r.Portions)), positive(r.Pot)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM recipe_items WHERE recipe_id = ? AND user_id = ?", id, uid); err != nil {
		return err
	}
	for i := range r.Items {
		it := &r.Items[i]
		n := it.Nutrients
		o := optionalNutrients(&n)
		q := it.Quantity
		owner, foodID := ingredientFood(userID, it)
		if _, err := tx.ExecContext(ctx, `INSERT INTO recipe_items (recipe_id, user_id, position, food_user_id, food_id, name, brand,
			quantity_amount, quantity_unit, quantity_servings, quantity_serving, quantity_portions, amount, unit,
			kcal, protein, carbs, fat, fiber, saturated_fat, sugars, salt, polyols, alcohol, source, note, portion_grams)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, uid, i, owner, foodID, it.Name, it.Brand,
			q.Amount, q.Unit, q.Servings, q.Serving, q.Portions, it.Amount, it.Unit,
			n.Kcal, n.Protein, n.Carbs, n.Fat, nullable(*o[0]), nullable(*o[1]), nullable(*o[2]), nullable(*o[3]),
			nullable(*o[4]), nullable(*o[5]), string(it.Source), it.Note, positive(it.PortionGrams)); err != nil {
			return err
		}
	}
	return nil
}

// ingredientFood is the owner and the id of the food of userID's
// recipe's ingredient it, as recipe_items keeps them: userID for a food
// of theirs, the member it names for another's (meal.Item.FoodUser), and
// both NULL for a food gone (its owner's account deleted).
func ingredientFood(userID uuid.UUID, it *meal.Item) (any, any) {
	switch {
	case it.FoodID == uuid.Nil():
		return nil, nil
	case it.FoodUser != uuid.Nil():
		return it.FoodUser.String(), it.FoodID.String()
	}
	return userID.String(), it.FoodID.String()
}

// positive is v for a column that is NULL for none: NULL for 0.
func positive(v float64) any {
	if v == 0 {
		return nil
	}
	return v
}

// RecipeByID returns userID's recipe of the food with id, archived or
// not, or ErrNotFound: for a food that is no recipe too, and for a
// recipe they left the share of.
func (db *DB) RecipeByID(ctx context.Context, userID, id uuid.UUID) (recipe.Recipe, error) {
	var r recipe.Recipe
	err := db.readTx(ctx, func(tx *sql.Tx) error {
		left, err := withdrawn(ctx, tx, userID, id)
		if err != nil {
			return err
		}
		if left {
			return ErrNotFound
		}
		r, err = recipeInTx(ctx, tx, userID, id)
		return err
	})
	if errors.Is(err, ErrNotFound) {
		return recipe.Recipe{}, err
	}
	if err != nil {
		return recipe.Recipe{}, fmt.Errorf("recipe: %w", err)
	}
	return r, nil
}

// recipeInTx reads userID's recipe id in tx, its food and ingredients, or
// returns ErrNotFound.
func recipeInTx(ctx context.Context, tx *sql.Tx, userID, id uuid.UUID) (recipe.Recipe, error) {
	var yield, pot sql.NullFloat64
	var portions sql.NullInt64
	err := tx.QueryRowContext(ctx, "SELECT yield_g, portions, pot_g FROM recipes WHERE user_id = ? AND food_id = ?",
		userID.String(), id.String()).Scan(&yield, &portions, &pot)
	if errors.Is(err, sql.ErrNoRows) {
		return recipe.Recipe{}, ErrNotFound
	}
	if err != nil {
		return recipe.Recipe{}, err
	}
	r := recipe.Recipe{Yield: yield.Float64, Portions: int(portions.Int64), Pot: pot.Float64}
	if r.Food, err = foodInTx(ctx, tx, userID, id); err != nil {
		return recipe.Recipe{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT food_user_id, food_id, name, brand, quantity_amount, quantity_unit, quantity_servings,
		quantity_serving, quantity_portions, amount, unit, kcal, protein, carbs, fat,
		fiber, saturated_fat, sugars, salt, polyols, alcohol, source, note, portion_grams
		FROM recipe_items WHERE user_id = ? AND recipe_id = ? ORDER BY position`, userID.String(), id.String())
	if err != nil {
		return recipe.Recipe{}, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var it meal.Item
		var owner, foodID sql.NullString
		var source string
		var opt [6]sql.NullFloat64
		var portionGrams sql.NullFloat64
		q, n := &it.Quantity, &it.Nutrients
		if err := rows.Scan(&owner, &foodID, &it.Name, &it.Brand, &q.Amount, &q.Unit, &q.Servings, &q.Serving, &q.Portions,
			&it.Amount, &it.Unit, &n.Kcal, &n.Protein, &n.Carbs, &n.Fat,
			&opt[0], &opt[1], &opt[2], &opt[3], &opt[4], &opt[5], &source, &it.Note, &portionGrams); err != nil {
			return recipe.Recipe{}, err
		}
		it.PortionGrams = portionGrams.Float64
		if it.FoodID, it.FoodUser, err = ingredientIDs(userID, owner, foodID); err != nil {
			return recipe.Recipe{}, err
		}
		for i, p := range optionalNutrients(n) {
			if opt[i].Valid {
				v := opt[i].Float64
				*p = &v
			}
		}
		it.Source = food.Source(source)
		r.Items = append(r.Items, it)
	}
	return r, rows.Err()
}

// ingredientIDs reads an ingredient's food of userID's recipe: its id,
// and its owner when that is another member (meal.Item.FoodUser); both
// uuid.Nil for a food gone.
func ingredientIDs(userID uuid.UUID, owner, foodID sql.NullString) (uuid.UUID, uuid.UUID, error) {
	if !foodID.Valid {
		return uuid.Nil(), uuid.Nil(), nil
	}
	id, err := uuid.Parse(foodID.String)
	if err != nil {
		return uuid.Nil(), uuid.Nil(), fmt.Errorf("ingredient food id %q: %w", foodID.String, err)
	}
	if owner.String == userID.String() {
		return id, uuid.Nil(), nil
	}
	who, err := uuid.Parse(owner.String)
	if err != nil {
		return uuid.Nil(), uuid.Nil(), fmt.Errorf("ingredient food's user %q: %w", owner.String, err)
	}
	return id, who, nil
}

// RecipesOf returns the recipes of userID's foods among ids, by id, with
// their food and ingredients: the foods of a search or a list that are
// recipes.
func (db *DB) RecipesOf(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]recipe.Recipe, error) {
	out := map[uuid.UUID]recipe.Recipe{}
	err := db.readTx(ctx, func(tx *sql.Tx) error {
		for _, id := range ids {
			r, err := recipeInTx(ctx, tx, userID, id)
			if errors.Is(err, ErrNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			out[id] = r
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("recipes: %w", err)
	}
	return out, nil
}

// checkRecipeFood refuses a save of food f, which was before (nil for a
// new one), when it is or makes a recipe's food: only the recipe changes
// that, but for barcodes added to it (linkBarcodes) and whether it is
// archived, which leave its values as they are.
func checkRecipeFood(before *food.Food, f food.Food) error {
	switch {
	case before == nil && f.Source == food.Recipe:
		return ErrRecipeFood
	case before == nil || (before.Source != food.Recipe && f.Source != food.Recipe):
		return nil
	}
	had, now := *before, f
	had.Barcodes, now.Barcodes, had.Archived, now.Archived = nil, nil, false, false
	if !reflect.DeepEqual(had, now) {
		return ErrRecipeFood
	}
	return nil
}

// foodInUse reports whether a meal's item or a recipe's ingredient, a
// shared recipe's of another member's too, is of userID's food id, which then stays, archived, when undo would take it
// away.
func foodInUse(ctx context.Context, tx *sql.Tx, userID, id uuid.UUID) (bool, error) {
	var used int
	err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM meal_items WHERE user_id = ?1 AND food_id = ?2)
		OR EXISTS (SELECT 1 FROM recipe_items WHERE food_user_id = ?1 AND food_id = ?2)`,
		userID.String(), id.String()).Scan(&used)
	return used != 0, err
}

// takeBackRecipe puts the recipe of e back as it was before: its food
// and ingredients, or, for one e added, none: its food goes, unless a
// meal or another recipe uses it, which keeps it, archived, with its
// ingredients.
func takeBackRecipe(ctx context.Context, tx *sql.Tx, userID uuid.UUID, e event.Event, now time.Time) error {
	id, err := entityID(e)
	if err != nil {
		return err
	}
	if e.Before == nil {
		// Its other members' recipes would stay, of no one's making.
		if shared, err := sharedWithOthers(ctx, tx, userID, id); err != nil {
			return err
		} else if shared {
			return clash{UndoShared}
		}
		used, err := foodInUse(ctx, tx, userID, id)
		if err != nil {
			return err
		}
		if used {
			return setArchived(ctx, tx, userID, id, true, now)
		}
		_, err = tx.ExecContext(ctx, "DELETE FROM foods WHERE user_id = ? AND id = ?", userID.String(), id.String())
		return err
	}
	var r recipe.Recipe
	if err := decode(e.Before, &r); err != nil {
		return err
	}
	if err := checkFoodClash(ctx, tx, userID, r.Food); err != nil {
		return err
	}
	// An ingredient may be made of the recipe since (an import's save,
	// which undo leaves).
	if err := checkCycle(ctx, tx, userID, r.Food.ID, r.Items); errors.Is(err, ErrRecipeCycle) {
		return fmt.Errorf("%w: %w", clash{UndoStuck}, err)
	} else if err != nil {
		return err
	}
	if err := updateFood(ctx, tx, userID, r.Food, food.Key(r.Food.Name), food.Key(r.Food.Brand), now); err != nil {
		return err
	}
	if err := writeRecipe(ctx, tx, userID, r); err != nil {
		return err
	}
	return restoreShared(ctx, tx, userID, id, r, e, now)
}

// restoreShared puts back every member's recipe of userID's recipe id,
// which was the one undone (stateNow), as r, e's state before, and the
// share's version with them: the save before is the latest again.
func restoreShared(ctx context.Context, tx *sql.Tx, userID, id uuid.UUID, r recipe.Recipe, e event.Event, now time.Time) error {
	if err := syncShared(ctx, tx, userID, id, r, now); errors.Is(err, ErrShareNameTaken) {
		return fmt.Errorf("%w: %w", clash{UndoNameTaken}, err)
	} else if err != nil {
		return err
	}
	var was recipeState
	if err := decode(e.Before, &was); err != nil {
		return err
	}
	share, _, err := shareOf(ctx, tx, userID, id)
	if err != nil || share == "" || was.ShareVersion == 0 {
		return err
	}
	return setShareVersion(ctx, tx, share, was.ShareVersion)
}

// recipeStateNow returns the recipe of e now, as e records it, or nil
// when it is gone.
func recipeStateNow(ctx context.Context, tx *sql.Tx, userID uuid.UUID, e event.Event) (any, error) {
	id, err := entityID(e)
	if err != nil {
		return nil, err
	}
	// A recipe its user left is gone to them: its changes stay.
	if left, err := withdrawn(ctx, tx, userID, id); err != nil || left {
		return nil, err
	}
	r, err := recipeInTx(ctx, tx, userID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := recipeState{Recipe: r}
	if share, _, err := shareOf(ctx, tx, userID, id); err != nil {
		return nil, err
	} else if share != "" {
		if s.ShareVersion, err = shareVersion(ctx, tx, share); err != nil {
			return nil, err
		}
	}
	return s, nil
}
