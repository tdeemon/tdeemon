package api_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/tdeemon/tdeemon/internal/auth"
	"github.com/tdeemon/tdeemon/internal/domain/food"
	"github.com/tdeemon/tdeemon/internal/domain/user"
	"github.com/tdeemon/tdeemon/internal/store"
)

// sessionOfUser is a web session of u's, made in the store.
func sessionOfUser(t *testing.T, in *instance, u user.User) []*http.Cookie {
	t.Helper()
	now := in.clock.Now()
	token, id := auth.NewSessionToken()
	s := store.Session{ID: id, UserID: u.ID, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(auth.SessionLifetime)}
	if err := in.db.CreateSession(t.Context(), s); err != nil {
		t.Fatal(err)
	}
	return []*http.Cookie{sessionRequestCookie(token)}
}

// foodOf is u's food of name, failing the test without one.
func foodOf(t *testing.T, db *store.DB, u uuid.UUID, name string) food.Food {
	t.Helper()
	f, err := db.FoodByKey(t.Context(), u, name, "")
	if err != nil {
		t.Fatalf("%v's %q: %v", u, name, err)
	}
	return f
}

// Ana shares her pudding with Zoe by her address: Zoe finds it, her
// recipe lists both, its ingredient Ana's; a stop takes it from her, and
// refusals say why.
func TestShareARecipe(t *testing.T) {
	t.Parallel()

	in := newInstance(t, "http://localhost", "")
	ana := []*http.Cookie{sessionOf(t, in)}
	zoe := in.addUser(t, user.RoleUser)
	zoes := sessionOfUser(t, in, zoe)
	now := in.clock.Now()
	saveIsolationRecipe(t, in.db, in.user, saveIsolationFoods(t, in.db, in.user, now), now)
	pudding := foodOf(t, in.db, in.user.ID, in.user.Name+"'s bread pudding")
	path := "/api/v1/recipes/" + pudding.ID.String() + "/shares"

	// An address as typed, in any case.
	rec := in.do(t, http.MethodPost, path, `{"email": " Zoe@Example.com "}`, ana, nil)
	if body := rec.Body.String(); rec.Code != http.StatusOK || !strings.Contains(body, `"shared_with":[`) ||
		!strings.Contains(body, `"name":"Zoe"`) || strings.Contains(body, "zoe@example.com") {
		t.Fatalf("the share: %d %s", rec.Code, body)
	}
	theirs := foodOf(t, in.db, zoe.ID, in.user.Name+"'s bread pudding")
	rec = in.do(t, http.MethodGet, "/api/v1/foods/"+theirs.ID.String(), "", zoes, nil)
	if body := rec.Body.String(); rec.Code != http.StatusOK || !strings.Contains(body, `"theirs":true`) || !strings.Contains(body, `"you":true`) {
		t.Errorf("Zoe's pudding: %d %s, want Ana's ingredient, and her among its members", rec.Code, body)
	}

	for _, c := range []struct {
		body string
		want int
	}{
		{`{"email": "nobody@example.com"}`, http.StatusUnprocessableEntity},
		{`{"email": "` + in.user.Email + `"}`, http.StatusUnprocessableEntity},
		{`{"email": "zoe@example.com"}`, http.StatusConflict},
	} {
		if rec := in.do(t, http.MethodPost, path, c.body, ana, nil); rec.Code != c.want {
			t.Errorf("%s: %d %s, want %d", c.body, rec.Code, rec.Body.String(), c.want)
		}
	}

	members, err := in.db.RecipeMembers(t.Context(), in.user.ID, pudding.ID)
	if err != nil || len(members) != 2 {
		t.Fatalf("members = %v, %v", members, err)
	}
	stop := fmt.Sprintf("%s/%d", path, members[1].Seat)
	if rec := in.do(t, http.MethodDelete, stop, "", ana, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("the stop: %d %s", rec.Code, rec.Body.String())
	}
	if rec := in.do(t, http.MethodGet, "/api/v1/foods/"+theirs.ID.String(), "", zoes, nil); rec.Code != http.StatusNotFound {
		t.Errorf("Zoe's pudding after the stop: %d %s, want 404", rec.Code, rec.Body.String())
	}
	if rec := in.do(t, http.MethodDelete, stop, "", ana, nil); rec.Code != http.StatusNotFound {
		t.Errorf("the stop again: %d %s, want 404", rec.Code, rec.Body.String())
	}
}

// A shared recipe's ingredient of another member's food is added to
// one's foods, without its owner's words; one's own is no such
// ingredient; a name one has is a conflict. A save through the API keeps
// another's ingredient by its food_id.
func TestCopyAnIngredientAndSaveWithIt(t *testing.T) {
	t.Parallel()

	in := newInstance(t, "http://localhost", "")
	ana := []*http.Cookie{sessionOf(t, in)}
	zoe := in.addUser(t, user.RoleUser)
	zoes := sessionOfUser(t, in, zoe)
	now := in.clock.Now()
	bread := saveIsolationFoods(t, in.db, in.user, now)
	saveIsolationShare(t, in.db, in.user, zoe, bread, now)
	stew := foodOf(t, in.db, zoe.ID, "Shared bread stew")
	copyPath := "/api/v1/recipes/" + stew.ID.String() + "/ingredients/0/food"

	rec := in.do(t, http.MethodPost, copyPath, "", zoes, nil)
	if body := rec.Body.String(); rec.Code != http.StatusCreated || !strings.Contains(body, `"name":"`+bread.Name+`"`) ||
		strings.Contains(body, "loaf") || strings.Contains(body, "3850000000015") {
		t.Fatalf("the copy: %d %s, want the bread without Ana's alias or barcode", rec.Code, body)
	}
	if rec := in.do(t, http.MethodPost, copyPath, "", zoes, nil); rec.Code != http.StatusConflict {
		t.Errorf("the copy again: %d %s, want 409", rec.Code, rec.Body.String())
	}
	anas := foodOf(t, in.db, in.user.ID, "Shared bread stew")
	if rec := in.do(t, http.MethodPost, "/api/v1/recipes/"+anas.ID.String()+"/ingredients/0/food", "", ana, nil); rec.Code != http.StatusNotFound {
		t.Errorf("Ana's copy of her own: %d %s, want 404", rec.Code, rec.Body.String())
	}

	save := fmt.Sprintf(`{"id": %q, "name": "Shared bread stew", "items": [{"food_id": %q, "amount": 250, "unit": "g"}], "yield_g": 650}`,
		stew.ID, bread.ID)
	if rec := in.do(t, http.MethodPost, "/api/v1/recipes", save, zoes, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"theirs":true`) {
		t.Errorf("Zoe's save with Ana's bread: %d %s", rec.Code, rec.Body.String())
	}
	if r, err := in.db.RecipeByID(t.Context(), in.user.ID, anas.ID); err != nil || r.Yield != 650 || r.Items[0].Amount != 250 {
		t.Errorf("Ana's stew after Zoe's save = %+v, %v; want Zoe's", r, err)
	}
}

// A save of a shared recipe under a name another member's food has is a
// conflict, naming none of theirs; a food of another member's the recipe
// has not is none to name.
func TestASharedRecipeSaveRefuses(t *testing.T) {
	t.Parallel()

	in := newInstance(t, "http://localhost", "")
	ana := []*http.Cookie{sessionOf(t, in)}
	zoe := in.addUser(t, user.RoleUser)
	zoes := sessionOfUser(t, in, zoe)
	now := in.clock.Now()
	bread := saveIsolationFoods(t, in.db, in.user, now)
	saveIsolationShare(t, in.db, in.user, zoe, bread, now)
	stew := foodOf(t, in.db, zoe.ID, "Shared bread stew")
	anas := foodOf(t, in.db, in.user.ID, "Shared bread stew")
	// A name another member's food has is a conflict, naming none of theirs.
	zoeFood := food.Food{
		ID: uuid.NewV7(), Name: "Zoe's soup", Basis: food.Per100g, Source: food.Estimate,
		Nutrients: food.Nutrients{Kcal: 50, Protein: 2, Carbs: 6, Fat: 2},
	}
	if _, err := in.db.SaveFood(t.Context(), zoe.ID, isolationActor, zoeFood, now); err != nil {
		t.Fatal(err)
	}
	rename := fmt.Sprintf(`{"id": %q, "name": "Zoe's soup", "items": [{"food_id": %q, "amount": 250, "unit": "g"}]}`, anas.ID, bread.ID)
	if rec := in.do(t, http.MethodPost, "/api/v1/recipes", rename, ana, nil); rec.Code != http.StatusConflict || strings.Contains(rec.Body.String(), zoeFood.ID.String()) {
		t.Errorf("Ana's rename to Zoe's food's name: %d %s, want 409 naming none of hers", rec.Code, rec.Body.String())
	}
	// Ana's food unknown to the stew is none of Zoe's to name.
	other := food.Food{
		ID: uuid.NewV7(), Name: "Ana's oats", Basis: food.Per100g, Source: food.Label,
		Nutrients: food.Nutrients{Kcal: 380, Protein: 13, Carbs: 60, Fat: 7},
	}
	if _, err := in.db.SaveFood(t.Context(), in.user.ID, isolationActor, other, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	bad := fmt.Sprintf(`{"id": %q, "name": "Shared bread stew", "items": [{"food_id": %q, "amount": 100, "unit": "g"}]}`, stew.ID, other.ID)
	if rec := in.do(t, http.MethodPost, "/api/v1/recipes", bad, zoes, nil); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("Zoe's save with another food of Ana's: %d %s, want 422", rec.Code, rec.Body.String())
	}
}

// Sharing is a web session's: a token's is forbidden.
func TestShareIsClosedToTokens(t *testing.T) {
	t.Parallel()

	in, session := signedIn(t)
	tok := mcpToken(t, in, session, `"tools"`)
	if rec := in.do(t, http.MethodPost, "/api/v1/recipes/"+uuid.NewV7().String()+"/shares", `{"email": "x@example.com"}`, nil, bearer(tok)); rec.Code != http.StatusForbidden {
		t.Errorf("a token's share: %d %s, want 403", rec.Code, rec.Body.String())
	}
}
