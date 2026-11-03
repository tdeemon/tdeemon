package api_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/food"
	"github.com/zarnautovic/tdeemon/internal/domain/meal"
	"github.com/zarnautovic/tdeemon/internal/domain/user"
	"github.com/zarnautovic/tdeemon/internal/store"
)

// householdAnswer is get-household's answer.
type householdAnswer struct {
	Members  []map[string]any `json:"members"`
	Sent     []inviteAnswer   `json:"sent"`
	Received []inviteAnswer   `json:"received"`
}

type inviteAnswer struct {
	ID string `json:"id"`
	By struct {
		Name, Email string
	} `json:"by"`
	Invited struct {
		Name, Email string
	} `json:"invited"`
}

// problemAnswer is a problem's detail and its errors' locations and
// values.
type problemAnswer struct {
	Detail string `json:"detail"`
	Errors []struct {
		Location string `json:"location"`
		Value    any    `json:"value"`
	} `json:"errors"`
}

// codeOf is the code of a household's refusal in rec: its error's value
// at where.
func codeOf(t *testing.T, rec interface{ Bytes() []byte }, where string) string {
	t.Helper()
	var p problemAnswer
	if err := json.Unmarshal(rec.Bytes(), &p); err != nil {
		t.Fatalf("decode %s: %v", rec.Bytes(), err)
	}
	for _, e := range p.Errors {
		if s, ok := e.Value.(string); ok && e.Location == where {
			return s
		}
	}
	return ""
}

func getHousehold(t *testing.T, in *instance, session []*http.Cookie) householdAnswer {
	t.Helper()
	rec := in.do(t, http.MethodGet, "/api/v1/household", "", session, nil)
	var got householdAnswer
	if err := json.Unmarshal(rec.Body.Bytes(), &got); rec.Code != http.StatusOK || err != nil {
		t.Fatalf("the household: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("the household's Cache-Control = %q, want no-store", rec.Header().Get("Cache-Control"))
	}
	return got
}

// inviteByAPI is session's invite of address into their household, and
// its id.
func inviteByAPI(t *testing.T, in *instance, session []*http.Cookie, address string) string {
	t.Helper()
	rec := in.do(t, http.MethodPost, "/api/v1/household/invites", `{"email": "`+address+`"}`, session, nil)
	var got inviteAnswer
	if err := json.Unmarshal(rec.Body.Bytes(), &got); rec.Code != http.StatusCreated || err != nil {
		t.Fatalf("the invite of %s: %d %s", address, rec.Code, rec.Body.String())
	}
	return got.ID
}

// previewByAPI is session's preview of the invite id: its fingerprint.
func previewByAPI(t *testing.T, in *instance, session []*http.Cookie, id string) string {
	t.Helper()
	rec := in.do(t, http.MethodGet, "/api/v1/household/invites/"+id+"/preview", "", session, nil)
	var got struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); rec.Code != http.StatusOK || err != nil || got.Fingerprint == "" {
		t.Fatalf("the preview: %d %s", rec.Code, rec.Body.String())
	}
	return got.Fingerprint
}

func acceptBody(fingerprint string) string { return `{"fingerprint": "` + fingerprint + `"}` }

// Ana invites Zoe, who is told and sees the invite; she previews and
// joins, and Ana is told; each sees the other's name and address alone;
// Mia, outside, sees nothing of it. Zoe leaves, and Ana is told.
func TestHouseholdInviteJoinAndLeave(t *testing.T) {
	t.Parallel()

	in := newInstance(t, "http://localhost", "")
	ana := []*http.Cookie{sessionOf(t, in)}
	zoe := in.addUser(t, user.RoleUser)
	zoes := sessionOfUser(t, in, zoe)
	now := in.clock.Now()
	mia := newIsolationUser(t, in.db, "mia@example.com", "Mia", now)
	mias := sessionOfUser(t, in, user.User{ID: mia})
	subscribe(t, in, ana[0], "https://web.push.apple.com/ana", "")
	subscribe(t, in, zoes[0], "https://web.push.apple.com/zoe", "")
	saveIsolationFoods(t, in.db, in.user, now)
	in.pushes.endpoints()

	id := inviteByAPI(t, in, ana, " Zoe@Example.com ")
	told(t, in, "the invite", "https://web.push.apple.com/zoe")
	waitsForZoe(t, in, id, ana, zoes, mias)

	rec := in.do(t, http.MethodGet, "/api/v1/household/invites/"+id+"/preview", "", zoes, nil)
	if body := rec.Body.String(); rec.Code != http.StatusOK || !strings.Contains(body, `"received":{"foods":2,"recipes":0}`) {
		t.Errorf("Zoe's preview: %d %s, want Ana's two foods to come to her", rec.Code, body)
	}
	fingerprint := previewByAPI(t, in, zoes, id)
	rec = in.do(t, http.MethodPost, "/api/v1/household/invites/"+id+"/accept", acceptBody(fingerprint), zoes, nil)
	var joined householdAnswer
	if err := json.Unmarshal(rec.Body.Bytes(), &joined); rec.Code != http.StatusOK || err != nil || len(joined.Members) != 2 {
		t.Fatalf("Zoe's accept: %d %s", rec.Code, rec.Body.String())
	}
	told(t, in, "the accept", "https://web.push.apple.com/ana")
	for _, who := range [][]*http.Cookie{ana, zoes} {
		anaAndZoe(t, getHousehold(t, in, who))
	}
	if got := getHousehold(t, in, mias); len(got.Members)+len(got.Sent)+len(got.Received) != 0 {
		t.Errorf("Mia's household = %+v, want nothing", got)
	}
	if rec := in.do(t, http.MethodPost, "/api/v1/household/leave", "", mias, nil); rec.Code != http.StatusNotFound {
		t.Errorf("Mia's leave: %d %s, want 404", rec.Code, rec.Body.String())
	}

	if rec := in.do(t, http.MethodPost, "/api/v1/household/leave", "", zoes, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("Zoe's leave: %d %s", rec.Code, rec.Body.String())
	}
	told(t, in, "the leave", "https://web.push.apple.com/ana")
	if got := getHousehold(t, in, ana); len(got.Members) != 0 {
		t.Errorf("Ana's household after Zoe left = %+v, want none: a household of one is no more", got)
	}
	if rec := in.do(t, http.MethodPost, "/api/v1/household/leave", "", zoes, nil); rec.Code != http.StatusNotFound {
		t.Errorf("Zoe's leave again: %d %s, want 404", rec.Code, rec.Body.String())
	}
}

// waitsForZoe fails unless Ana's invite id waits for Zoe's answer, as
// each of them sees it, and is Zoe's alone to answer, not Ana's nor Mia's.
func waitsForZoe(t *testing.T, in *instance, id string, ana, zoes, mias []*http.Cookie) {
	t.Helper()
	if got := getHousehold(t, in, zoes); len(got.Members) != 0 || len(got.Received) != 1 || got.Received[0].ID != id ||
		got.Received[0].By.Email != "ana@example.com" || len(got.Sent) != 0 {
		t.Errorf("Zoe's household = %+v, want Ana's invite for her answer", got)
	}
	if got := getHousehold(t, in, ana); len(got.Sent) != 1 || got.Sent[0].Invited.Name != "Zoe" || len(got.Received) != 0 {
		t.Errorf("Ana's household = %+v, want her invite of Zoe", got)
	}
	// The invite is Zoe's to preview and accept, no one else's.
	notTheirs(t, in, id, ana, mias)
	if rec := in.do(t, http.MethodDelete, "/api/v1/household/invites/"+id, "", mias, nil); rec.Code != http.StatusNotFound {
		t.Errorf("Mia's delete of the invite: %d %s, want 404", rec.Code, rec.Body.String())
	}
}

// told fails unless what told the devices of endpoints alone since the
// last look.
func told(t *testing.T, in *instance, what string, endpoints ...string) {
	t.Helper()
	if got := in.pushes.endpoints(); !slices.Equal(got, endpoints) {
		t.Errorf("%s told %v, want %v", what, got, endpoints)
	}
}

// notTheirs fails unless the invite id is none of each of whos' to
// preview or accept.
func notTheirs(t *testing.T, in *instance, id string, whos ...[]*http.Cookie) {
	t.Helper()
	path := "/api/v1/household/invites/" + id
	for _, who := range whos {
		if rec := in.do(t, http.MethodGet, path+"/preview", "", who, nil); rec.Code != http.StatusNotFound {
			t.Errorf("a preview of another's invite: %d %s, want 404", rec.Code, rec.Body.String())
		}
		if rec := in.do(t, http.MethodPost, path+"/accept", acceptBody("x"), who, nil); rec.Code != http.StatusNotFound {
			t.Errorf("an accept of another's invite: %d %s, want 404", rec.Code, rec.Body.String())
		}
	}
}

// anaAndZoe fails unless got is a household of Ana, then Zoe, each by
// their name and address alone, and no invites.
func anaAndZoe(t *testing.T, got householdAnswer) {
	t.Helper()
	if len(got.Members) != 2 || len(got.Sent) != 0 || len(got.Received) != 0 {
		t.Fatalf("the household = %+v, want Ana and Zoe, no invites", got)
	}
	for _, m := range got.Members {
		for k := range m {
			if k != "name" && k != "email" && k != "you" {
				t.Errorf("a member shows %q: %v; want their name and address alone", k, m)
			}
		}
	}
	if got.Members[0]["email"] != "ana@example.com" || got.Members[1]["email"] != "zoe@example.com" {
		t.Errorf("the members = %v, want Ana, then Zoe", got.Members)
	}
}

// Each refusal of an invite says why by its code.
func TestHouseholdInviteRefusals(t *testing.T) {
	t.Parallel()

	in := newInstance(t, "http://localhost", "")
	ana := []*http.Cookie{sessionOf(t, in)}
	now := in.clock.Now()
	zoe := in.addUser(t, user.RoleUser)
	zoes := sessionOfUser(t, in, zoe)
	newIsolationUser(t, in.db, "eva@example.com", "Eva", now)
	gone := newIsolationUser(t, in.db, "gone@example.com", "Gone", now)
	if _, err := in.db.ChangeUser(t.Context(), gone, store.UserChange{Active: new(false)}, now); err != nil {
		t.Fatal(err)
	}
	mia := newIsolationUser(t, in.db, "mia@example.com", "Mia", now)
	lea := newIsolationUser(t, in.db, "lea@example.com", "Lea", now)
	inHousehold(t, in.db, mia, lea)
	inHousehold(t, in.db, in.user.ID, zoe.ID)
	inviteByAPI(t, in, zoes, "eva@example.com")
	in.pushes.endpoints()

	for _, c := range []struct {
		email  string
		status int
		code   string
	}{
		{"nobody@example.com", http.StatusUnprocessableEntity, "no_user"},
		{"ANA@example.com", http.StatusUnprocessableEntity, "self"},
		{"gone@example.com", http.StatusUnprocessableEntity, "deactivated"},
		{"zoe@example.com", http.StatusConflict, "member"},
		{"mia@example.com", http.StatusConflict, "other_household"},
		// Zoe's invite is the household's: Ana's waits behind it.
		{"eva@example.com", http.StatusConflict, "waiting"},
	} {
		rec := in.do(t, http.MethodPost, "/api/v1/household/invites", `{"email": "`+c.email+`"}`, ana, nil)
		if rec.Code != c.status || codeOf(t, rec.Body, "body.email") != c.code {
			t.Errorf("an invite of %s: %d %s, want %d %s", c.email, rec.Code, rec.Body.String(), c.status, c.code)
		}
	}
	if got := in.pushes.endpoints(); len(got) != 0 {
		t.Errorf("the refusals told %v, want no one", got)
	}
}

// inHousehold puts users in a household of their own, straight in the
// tables: no merge, so each keeps foods the others have none of.
func inHousehold(t *testing.T, db *store.DB, users ...uuid.UUID) {
	t.Helper()
	at := time.Date(2026, 10, 21, 8, 0, 0, 0, time.UTC).Format(time.RFC3339)
	h := uuid.NewV7().String()
	err := db.WriteTx(t.Context(), func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(t.Context(), "INSERT INTO households (id, created_at) VALUES (?, ?)", h, at); err != nil {
			return err
		}
		for _, u := range users {
			if _, err := tx.ExecContext(t.Context(), "INSERT INTO household_members (user_id, household_id, joined_at) VALUES (?, ?, ?)",
				u.String(), h, at); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// An accept of a merge its preview showed no more does nothing, and says
// so by its code; the new preview's is accepted. An invite from a
// household Zoe can join no more is refused at its preview and accept.
func TestHouseholdAcceptAfterAChange(t *testing.T) {
	t.Parallel()

	in := newInstance(t, "http://localhost", "")
	ana := []*http.Cookie{sessionOf(t, in)}
	zoe := in.addUser(t, user.RoleUser)
	zoes := sessionOfUser(t, in, zoe)
	now := in.clock.Now()
	id := inviteByAPI(t, in, ana, zoe.Email)
	// Mia's invite of Zoe waits too, made while Zoe was in no household.
	mia := newIsolationUser(t, in.db, "mia@example.com", "Mia", now)
	inv, err := in.db.InviteToHousehold(t.Context(), mia, zoe.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	old := previewByAPI(t, in, zoes, id)
	saveIsolationFoods(t, in.db, in.user, now)

	path := "/api/v1/household/invites/" + id + "/accept"
	rec := in.do(t, http.MethodPost, path, acceptBody(old), zoes, nil)
	if rec.Code != http.StatusConflict || codeOf(t, rec.Body, "body.fingerprint") != "merge_changed" {
		t.Errorf("the accept of the old preview: %d %s, want 409 merge_changed", rec.Code, rec.Body.String())
	}
	if got := getHousehold(t, in, zoes); len(got.Members) != 0 || len(got.Received) != 2 {
		t.Errorf("Zoe's household after the refusal = %+v, want both invites still waiting", got)
	}
	fresh := previewByAPI(t, in, zoes, id)
	if fresh == old {
		t.Fatal("the new preview's fingerprint is the old one's")
	}
	if rec := in.do(t, http.MethodPost, path, acceptBody(fresh), zoes, nil); rec.Code != http.StatusOK {
		t.Errorf("the accept of the new preview: %d %s", rec.Code, rec.Body.String())
	}

	// Mia's invite waits, but Zoe, now in Ana's household, leaves it first.
	other := "/api/v1/household/invites/" + inv.ID.String()
	rec = in.do(t, http.MethodGet, other+"/preview", "", zoes, nil)
	if rec.Code != http.StatusConflict || codeOf(t, rec.Body, "path.id") != "other_household" {
		t.Errorf("Zoe's preview of Mia's invite: %d %s, want 409 other_household", rec.Code, rec.Body.String())
	}
	rec = in.do(t, http.MethodPost, other+"/accept", acceptBody(fresh), zoes, nil)
	if rec.Code != http.StatusConflict || codeOf(t, rec.Body, "path.id") != "other_household" {
		t.Errorf("Zoe's accept of Mia's invite: %d %s, want 409 other_household", rec.Code, rec.Body.String())
	}
}

// The invited user declines an invite, and the household revokes one,
// any member of it; neither twice.
func TestHouseholdInviteDeclineAndRevoke(t *testing.T) {
	t.Parallel()

	in := newInstance(t, "http://localhost", "")
	ana := []*http.Cookie{sessionOf(t, in)}
	zoe := in.addUser(t, user.RoleUser)
	zoes := sessionOfUser(t, in, zoe)
	now := in.clock.Now()
	eva := newIsolationUser(t, in.db, "eva@example.com", "Eva", now)
	evas := sessionOfUser(t, in, user.User{ID: eva})

	id := inviteByAPI(t, in, ana, zoe.Email)
	for _, want := range []int{http.StatusNoContent, http.StatusNotFound} {
		if rec := in.do(t, http.MethodDelete, "/api/v1/household/invites/"+id, "", zoes, nil); rec.Code != want {
			t.Errorf("Zoe's decline: %d %s, want %d", rec.Code, rec.Body.String(), want)
		}
	}
	inHousehold(t, in.db, in.user.ID, eva)
	id = inviteByAPI(t, in, ana, zoe.Email)
	for _, want := range []int{http.StatusNoContent, http.StatusNotFound} {
		if rec := in.do(t, http.MethodDelete, "/api/v1/household/invites/"+id, "", evas, nil); rec.Code != want {
			t.Errorf("Eva's revoke of Ana's invite: %d %s, want %d", rec.Code, rec.Body.String(), want)
		}
	}
	if rec := in.do(t, http.MethodDelete, "/api/v1/household/invites/not-an-id", "", evas, nil); rec.Code != http.StatusNotFound &&
		rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("a delete of no id: %d %s", rec.Code, rec.Body.String())
	}
}

// The household is a web session's: a token's request of any of its
// operations is forbidden.
func TestHouseholdIsClosedToTokens(t *testing.T) {
	t.Parallel()

	in, session := signedIn(t)
	tok := bearer(mcpToken(t, in, session, `"read","tools"`))
	id := uuid.NewV7().String()
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/household", ""},
		{http.MethodPost, "/api/v1/household/invites", `{"email": "zoe@example.com"}`},
		{http.MethodDelete, "/api/v1/household/invites/" + id, ""},
		{http.MethodGet, "/api/v1/household/invites/" + id + "/preview", ""},
		{http.MethodPost, "/api/v1/household/invites/" + id + "/accept", acceptBody("x")},
		{http.MethodPost, "/api/v1/household/leave", ""},
	} {
		if rec := in.do(t, c.method, c.path, c.body, nil, tok); rec.Code != http.StatusForbidden {
			t.Errorf("a token's %s %s: %d %s, want 403", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
}

// A save another member's food is in the way of, by its name or a
// barcode, is a conflict naming none of theirs: of a food, a meal's new
// food, a recipe, and a meal corrected; and a member does not take
// another out of a household's recipe, nor leave it alone.
func TestHouseholdClashesAreConflicts(t *testing.T) {
	t.Parallel()

	in := newInstance(t, "http://localhost", "")
	ana := []*http.Cookie{sessionOf(t, in)}
	zoe := in.addUser(t, user.RoleUser)
	now := in.clock.Now()
	// Zoe's foods of before the household, which no merge brought Ana.
	oats := food.Food{
		ID: uuid.NewV7(), Name: "Oats", Basis: food.Per100g, Source: food.Label, Barcodes: []string{"2000000000015"},
		Nutrients: food.Nutrients{Kcal: 380, Protein: 13, Carbs: 60, Fat: 7},
	}
	if _, err := in.db.SaveFood(t.Context(), zoe.ID, isolationActor, oats, now); err != nil {
		t.Fatal(err)
	}
	inHousehold(t, in.db, in.user.ID, zoe.ID)
	bread := saveIsolationFoods(t, in.db, in.user, now)

	newFood := `{"name": "Oats", "basis": "100g", "source": "label", "nutrients": {"kcal": 380, "protein": 13, "carbs": 60, "fat": 7}}`
	barcoded := `{"name": "Kefir", "basis": "100ml", "source": "label", "barcodes": ["2000000000015"], "nutrients": {"kcal": 60, "protein": 3, "carbs": 4, "fat": 3}}`
	mealID := meal.NewIDAt(now).String()
	for _, c := range []struct{ name, method, path, body string }{
		{"a new food", http.MethodPost, "/api/v1/foods/new", newFood},
		{"a food of a barcode", http.MethodPost, "/api/v1/foods/new", barcoded},
		{"a food upserted", http.MethodPost, "/api/v1/foods", newFood},
		{"a meal's new food", http.MethodPost, "/api/v1/meals", `{"items": [{"food": ` + newFood + `, "amount": 50, "unit": "g"}]}`},
		{"a recipe", http.MethodPost, "/api/v1/recipes/new", fmt.Sprintf(`{"name": "Oats", "items": [{"food_id": %q, "amount": 100, "unit": "g"}]}`, bread.ID)},
	} {
		rec := in.do(t, c.method, c.path, c.body, ana, nil)
		if rec.Code != http.StatusConflict || strings.Contains(rec.Body.String(), oats.ID.String()) {
			t.Errorf("%s: %d %s, want 409 naming none of Zoe's", c.name, rec.Code, rec.Body.String())
		}
	}
	rec := in.do(t, http.MethodPost, "/api/v1/meals", fmt.Sprintf(`{"id": %q, "items": [{"food_id": %q, "amount": 50, "unit": "g"}]}`, mealID, bread.ID), ana, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("Ana's meal: %d %s", rec.Code, rec.Body.String())
	}
	add := fmt.Sprintf(`{"ops": [{"op": "add", "item": {"id": %q, "food": %s, "amount": 50, "unit": "g"}}]}`, uuid.NewV4(), newFood)
	if rec := in.do(t, http.MethodPatch, "/api/v1/meals/"+mealID, add, ana, nil); rec.Code != http.StatusConflict {
		t.Errorf("a meal corrected with a new food: %d %s, want 409", rec.Code, rec.Body.String())
	}

	// Ana's recipe is the household's: she does not leave it alone.
	saveIsolationRecipe(t, in.db, in.user, bread, now)
	pudding := foodOf(t, in.db, in.user.ID, in.user.Name+"'s bread pudding")
	members, err := in.db.RecipeMembers(t.Context(), in.user.ID, pudding.ID)
	if err != nil || len(members) != 2 {
		t.Fatalf("the pudding's members = %+v, %v; want Ana and Zoe", members, err)
	}
	for _, m := range members {
		path := fmt.Sprintf("/api/v1/recipes/%s/shares/%d", pudding.ID, m.Seat)
		if rec := in.do(t, http.MethodDelete, path, "", ana, nil); rec.Code != http.StatusConflict {
			t.Errorf("Ana's stop of seat %d of the household's recipe: %d %s, want 409", m.Seat, rec.Code, rec.Body.String())
		}
	}
}

// Undo refuses a join, saying it takes nothing before it back either.
func TestUndoRefusesAJoin(t *testing.T) {
	t.Parallel()

	in := newInstance(t, "http://localhost", "")
	ana := []*http.Cookie{sessionOf(t, in)}
	zoe := in.addUser(t, user.RoleUser)
	zoes := sessionOfUser(t, in, zoe)
	id := inviteByAPI(t, in, ana, zoe.Email)
	if rec := in.do(t, http.MethodPost, "/api/v1/household/invites/"+id+"/accept", acceptBody(previewByAPI(t, in, zoes, id)), zoes, nil); rec.Code != http.StatusOK {
		t.Fatalf("the accept: %d %s", rec.Code, rec.Body.String())
	}
	rec := in.do(t, http.MethodPost, "/api/v1/undo", `{}`, zoes, nil)
	if body := rec.Body.String(); rec.Code != http.StatusConflict || !strings.Contains(body, "nothing before it") || strings.Contains(body, "passed over") {
		t.Errorf("the undo of the join: %d %s, want 409 taking nothing before it back", rec.Code, body)
	}
	rec = in.do(t, http.MethodGet, "/api/v1/changes", "", zoes, nil)
	if body := rec.Body.String(); rec.Code != http.StatusOK || !strings.Contains(body, `"action":"household.join","entity":"household"`) || strings.Contains(body, `"next":true`) {
		t.Errorf("Zoe's changes: %d %s, want the join, not next", rec.Code, body)
	}
}

// kitchenTables are the tables of a user's foods and recipes, whose rows a
// household's members each have a copy of.
var kitchenTables = []string{
	"foods", "foods_fts", "food_servings", "food_barcodes", "food_aliases", "food_shares", "recipes", "recipe_items", "catalog_saves",
}

// householdTables are the tables of the household itself: its members see
// one another's rows of them, as get-household shows the invites any
// member made.
var householdTables = []string{"households", "household_members", "household_invites"}

// Ana and Ivo join one household: Ivo's foods and recipes are Ana's
// copies of hers and the members' names and addresses are his to see, and
// nothing else of hers: no other row of his holds a value of hers, and no
// answer of a GET of his shows one, his export among them. Mia, outside,
// sees none of it.
func TestHouseholdSharesTheKitchenAlone(t *testing.T) {
	t.Parallel()

	e := newIsolationEnv(t, false)
	ana, ivo := e.owner, e.other
	ivos := []*http.Cookie{ivo.session}
	if _, err := e.in.db.LeaveHousehold(t.Context(), ivo.user.ID, isolationActor, e.in.clock.Now()); err != nil {
		t.Fatal(err)
	}
	before := map[uuid.UUID][]string{ana.user.ID: privateRows(t, e, ana.user.ID), ivo.user.ID: privateRows(t, e, ivo.user.ID)}
	id := inviteByAPI(t, e.in, []*http.Cookie{ana.session}, ivo.user.Email)
	if rec := e.in.do(t, http.MethodPost, "/api/v1/household/invites/"+id+"/accept", acceptBody(previewByAPI(t, e.in, ivos, id)), ivos, nil); rec.Code != http.StatusOK {
		t.Fatalf("Ivo's accept: %d %s", rec.Code, rec.Body.String())
	}
	// Neither's meals, measurements, plan or anything else out of the
	// kitchen changed, a number of them neither.
	samePrivateRows(t, e, before)

	forbidden := notToCross(t, e)
	shows := func(what, body string, values []string) {
		t.Helper()
		for _, v := range values {
			if strings.Contains(body, v) {
				t.Errorf("%s shows Ana's %q: %s", what, v, body)
			}
		}
	}
	mia := newIsolationUser(t, e.in.db, "mia@example.com", "Mia", e.in.clock.Now())
	mias := sessionOfUser(t, e.in, user.User{ID: mia})
	for _, op := range operations(t) {
		if op.method != http.MethodGet || strings.Contains(op.route, "{") {
			continue
		}
		for who, values := range map[string][]string{"Ivo": forbidden, "Mia": e.ownerValues} {
			session := ivos
			if who == "Mia" {
				session = mias
			}
			rec := e.in.do(t, http.MethodGet, op.path, "", session, nil)
			body := rec.Body.String()
			if op.id == "export-account" && rec.Code == http.StatusOK {
				body = ""
				for _, f := range unzipExport(t, rec.Body.Bytes()) {
					body += f
				}
			}
			shows(who+"'s GET "+op.route, body, values)
		}
	}
	rec := e.in.do(t, http.MethodGet, "/api/v1/foods/list", "", ivos, nil)
	if !strings.Contains(rec.Body.String(), ana.user.Name+"'s rye bread") {
		t.Errorf("Ivo's foods: %d %s, want Ana's bread among them", rec.Code, rec.Body.String())
	}
}

// notToCross are the owner's values the other, in a household with them,
// has none of: those the other's kitchen and household have now, and the
// members' names and addresses, cross. It fails if a row of the other's
// out of their kitchen and household holds one of the owner's values.
func notToCross(t *testing.T, e isolationEnv) []string {
	t.Helper()
	ana, ivo := e.owner, e.other
	// What may cross: the values of Ana's that Ivo's kitchen has now, and
	// the members' names and addresses.
	mate := strings.ToLower(ana.user.Name) + ".housemate@example.com"
	crosses := []string{ana.user.Name, ana.user.Email, ana.user.Name + "'s housemate", mate}
	var forbidden []string
	for _, tb := range e.userTables(t) {
		_, theirs := e.tableRows(t, tb, ivo.user.ID)
		if slices.Contains(householdTables, tb.name) {
			_, hers := e.tableRows(t, tb, ana.user.ID)
			crosses = append(crosses, append(hers, theirs...)...)
			continue
		}
		if slices.Contains(kitchenTables, tb.name) {
			crosses = append(crosses, theirs...)
			continue
		}
		for _, v := range theirs {
			for _, own := range e.ownerValues {
				if strings.Contains(v, own) && !slices.Contains([]string{ana.user.Name, ana.user.Email}, own) {
					t.Errorf("Ivo's %s holds Ana's %q: %s", tb.name, own, v)
				}
			}
		}
	}
	for _, v := range e.ownerValues {
		if !slices.Contains(crosses, v) {
			forbidden = append(forbidden, v)
		}
	}
	if !slices.Contains(crosses, ana.user.Name+"'s rye bread") {
		t.Fatalf("Ivo's kitchen has no copy of Ana's bread")
	}
	return forbidden
}

// samePrivateRows fails unless each user's privateRows are as before has
// them.
func samePrivateRows(t *testing.T, e isolationEnv, before map[uuid.UUID][]string) {
	t.Helper()
	for who, rows := range before {
		if after := privateRows(t, e, who); !slices.Equal(after, rows) {
			t.Errorf("the rows of %v out of the kitchen changed:\nbefore %s\nafter  %s", who,
				strings.Join(rows, "\n       "), strings.Join(after, "\n       "))
		}
	}
}

// lastSent is when a push subscription was last sent to.
var lastSent = regexp.MustCompile(` last_sent_at=\S+`)

// privateRows are the rows of id's out of their kitchen and household,
// whole, but what a request or the join itself writes: the sessions' and
// tokens' last use, the join's event, and when a device was told.
func privateRows(t *testing.T, e isolationEnv, id uuid.UUID) []string {
	t.Helper()
	var out []string
	for _, tb := range e.userTables(t) {
		if slices.Contains(kitchenTables, tb.name) || slices.Contains(householdTables, tb.name) ||
			tb.name == "sessions" || tb.name == "api_tokens" {
			continue
		}
		rows, _ := e.tableRows(t, tb, id)
		for _, r := range rows {
			if !strings.Contains(r, " operation=household.join ") {
				out = append(out, lastSent.ReplaceAllString(r, ""))
			}
		}
	}
	slices.Sort(out)
	return out
}
