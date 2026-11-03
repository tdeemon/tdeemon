package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"uuid"

	"github.com/danielgtaylor/huma/v2"

	"github.com/zarnautovic/tdeemon/internal/domain/user"
	"github.com/zarnautovic/tdeemon/internal/notify"
	"github.com/zarnautovic/tdeemon/internal/push"
	"github.com/zarnautovic/tdeemon/internal/store"
)

// A recipe shared with other users of the instance (story 403): each has
// it, logs it and changes it; a web session shares it, by the other
// user's address, and stops it, by their seat in the share.

// recipeMemberBody is a member of a shared recipe, as the others see
// them: their seat, by which the share names them, and their name.
type recipeMemberBody struct {
	Seat int    `json:"seat" doc:"What a stop names."`
	Name string `json:"name"`
	You  bool   `json:"you,omitempty" doc:"A stop of your seat leaves the recipe."`
	// Household names the members no stop takes out.
	Household bool `json:"household,omitempty" doc:"A member of your household: no stop takes them out, and your leave takes them with you, refused when no one else would stay."`
}

// The inputs name the recipe by its food's id, each its own field: huma
// describes no field of an unexported embedded struct.

type shareRecipeInput struct {
	ID   string `path:"id" format:"uuid" doc:"The recipe's food."`
	Body struct {
		Email string `json:"email" format:"email" maxLength:"254" doc:"The address of the user of the instance to share the recipe with."`
	}
}

type unshareRecipeInput struct {
	ID   string `path:"id" format:"uuid" doc:"The recipe's food."`
	Seat int    `path:"seat" minimum:"1" doc:"The member's seat, of the recipe's shared_with."`
}

type ingredientFoodInput struct {
	ID       string `path:"id" format:"uuid" doc:"The recipe's food."`
	Position int    `path:"position" minimum:"0" maximum:"49" doc:"The ingredient's place in the recipe, from 0."`
}

func (s *sessions) registerRecipeShares(a huma.API) {
	huma.Register(a, huma.Operation{
		OperationID: "share-recipe",
		Method:      http.MethodPost,
		Path:        Prefix + "/recipes/{id}/shares",
		Summary:     "Share a recipe with another user",
		Description: "Needs a web session. The user of the address has the recipe at once, a recipe of their own of it that a change by any member is every member's; " +
			"its ingredients of your foods are theirs to see in it, never among their foods. They are told by a notification on their devices. " +
			"Answers the recipe, with whom it is shared now.",
		Tags:   []string{"foods"},
		Errors: []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
	}, s.shareRecipe)
	huma.Register(a, huma.Operation{
		OperationID:   "unshare-recipe",
		Method:        http.MethodDelete,
		Path:          Prefix + "/recipes/{id}/shares/{seat}",
		Summary:       "Stop sharing a recipe with a member",
		Description:   "Needs a web session. Your own seat leaves the recipe: your meals of it stay, but you pick it no more. The others keep it.",
		Tags:          []string{"foods"},
		DefaultStatus: http.StatusNoContent,
		Errors:        []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict},
	}, s.unshareRecipe)
	huma.Register(a, huma.Operation{
		OperationID:   "copy-ingredient-food",
		Method:        http.MethodPost,
		Path:          Prefix + "/recipes/{id}/ingredients/{position}/food",
		Summary:       "Add another member's ingredient to your foods",
		Description:   "Needs a web session. A new food of yours of a shared recipe's ingredient of another member's food: its name, brand, values and servings. A food of yours of its name and brand is a conflict (409).",
		Tags:          []string{"foods"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict},
	}, s.copyIngredientFood)
}

func (s *sessions) shareRecipe(ctx context.Context, in *shareRecipeInput) (*foodOutputNoStore, error) {
	p, err := requireSession(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, huma.Error404NotFound("no such recipe")
	}
	// The instance's users know one another: an address of none says so.
	with, err := s.db.UserByEmail(ctx, user.NormalizeEmail(in.Body.Email))
	if errors.Is(err, store.ErrNotFound) {
		return nil, huma.Error422UnprocessableEntity("no user of this instance has that address")
	} else if err != nil {
		return nil, internalError("sharing the recipe failed", err)
	}
	if with.ID == p.User.ID {
		return nil, huma.Error422UnprocessableEntity("a recipe is shared with another user, not yourself")
	}
	err = s.db.ShareRecipe(ctx, p.User.ID, actorOf(p), id, with.ID, s.now())
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, huma.Error404NotFound("no such recipe")
	case errors.Is(err, store.ErrShareMember):
		return nil, huma.Error409Conflict("they have the recipe already")
	case errors.Is(err, store.ErrShareNameTaken):
		return nil, huma.Error409Conflict("they have a food of the recipe's name: they rename theirs, or you yours")
	case errors.Is(err, store.ErrShareBarcodeTaken):
		return nil, huma.Error409Conflict("a member of their household has another food of a barcode of the recipe's: they take it from theirs, or you from yours")
	case err != nil:
		return nil, internalError("sharing the recipe failed", err)
	}
	s.tellShared(ctx, p.User.ID, p.User.Name, with, id)
	return s.sharedRecipeAnswer(ctx, id)
}

// tellShared tells the user with, by a notification on their devices,
// that sharer (by their name) shared their recipe recipeID with them:
// its name, and with's recipe of it to open. One that fails is the
// logs', never the share's.
func (s *sessions) tellShared(ctx context.Context, sharerID uuid.UUID, sharer string, with user.User, recipeID uuid.UUID) {
	if s.push == nil {
		return
	}
	theirs, name, err := s.db.MembersRecipe(ctx, sharerID, recipeID, with.ID)
	if err != nil {
		s.logger.WarnContext(ctx, "telling of a share failed", "error", err)
		return
	}
	title, ok := sharedWords[with.Locale]
	if !ok {
		title = sharedWords["en"]
	}
	n := push.Notification{Title: fmt.Sprintf(title, sharer), Body: name, URL: "/foods/" + theirs.String(), Tag: "share-" + theirs.String()}
	if r := notify.Push(ctx, s.db, s.push, with, n, push.Message{TTL: 24 * 60 * 60}, s.now()); r.Err != nil {
		s.logger.WarnContext(ctx, "telling of a share failed", "user_id", with.ID.String(), "error", r.Err)
	}
}

// sharedWords are the title of a share's notification in each language,
// of the sharer's name.
var sharedWords = map[string]string{
	"en": "%s shared a recipe with you",
	"hr": "%s dijeli recept s tobom",
}

func (s *sessions) unshareRecipe(ctx context.Context, in *unshareRecipeInput) (*struct{}, error) {
	p, err := requireSession(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, huma.Error404NotFound("no such recipe")
	}
	err = s.db.UnshareRecipe(ctx, p.User.ID, actorOf(p), id, in.Seat, s.now())
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, huma.Error404NotFound("no such member of the recipe")
	case errors.Is(err, store.ErrShareHousehold):
		return nil, huma.Error409Conflict("a member of a household keeps its recipes, and a household does not leave one alone: " +
			"one who wants none of them leaves the household")
	case err != nil:
		return nil, internalError("stopping the share failed", err)
	}
	return nil, nil
}

func (s *sessions) copyIngredientFood(ctx context.Context, in *ingredientFoodInput) (*createdFoodOutput, error) {
	p, err := requireSession(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, huma.Error404NotFound("no such ingredient")
	}
	f, err := s.db.IngredientFood(ctx, p.User.ID, id, in.Position)
	if errors.Is(err, store.ErrNotFound) {
		return nil, huma.Error404NotFound("no such ingredient of another member's food")
	} else if err != nil {
		return nil, internalError("adding the food failed", err)
	}
	saved, err := s.db.CreateFood(ctx, p.User.ID, actorOf(p), f, s.now())
	var conflict *store.FoodConflictError
	switch {
	case errors.As(err, &conflict):
		return nil, huma.Error409Conflict(conflictDetail(conflict))
	case shareRefusal(err) != nil:
		return nil, shareRefusal(err)
	case err != nil:
		return nil, internalError("adding the food failed", err)
	}
	return &createdFoodOutput{Body: newFoodBody(&saved)}, nil
}

// sharedRecipeAnswer is the user's recipe of the food id, as get_food answers
// it.
func (s *sessions) sharedRecipeAnswer(ctx context.Context, id uuid.UUID) (*foodOutputNoStore, error) {
	return s.getFood(ctx, &getFoodInput{ID: id.String()})
}
