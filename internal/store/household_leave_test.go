package store_test

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/zarnautovic/tdeemon/internal/domain/day"
	"github.com/zarnautovic/tdeemon/internal/domain/event"
	"github.com/zarnautovic/tdeemon/internal/domain/meal"
	"github.com/zarnautovic/tdeemon/internal/domain/recipe"
	"github.com/zarnautovic/tdeemon/internal/store"
)

// addMia adds a fourth user, Mia, to db and returns her id.
func addMia(t *testing.T, db *store.DB) uuid.UUID {
	t.Helper()
	u := newUser("mia@example.com")
	if err := db.CreateUser(t.Context(), u, ""); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return u.ID
}

// leave is userID's leave of their household, failing the test on an
// error.
func leave(t *testing.T, db *store.DB, userID uuid.UUID) {
	t.Helper()
	if err := db.LeaveHousehold(t.Context(), userID, testActor, t0); err != nil {
		t.Fatalf("LeaveHousehold: %v", err)
	}
}

// householdSeats is the number of userID's seats of a household, live or
// left.
func householdSeats(t *testing.T, db *store.DB, userID uuid.UUID) int {
	t.Helper()
	var n int
	if err := db.Read().QueryRowContext(t.Context(), "SELECT count(*) FROM food_shares WHERE user_id = ? AND household = 1",
		userID.String()).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// alone fails the test unless userID is in no household and keeps no
// food of theirs in a group with anyone: their kitchen is theirs alone.
func alone(t *testing.T, db *store.DB, userID uuid.UUID) {
	t.Helper()
	if got := membersOf(t, db, userID); len(got) != 0 {
		t.Errorf("%v's household = %v, want none", userID, got)
	}
	if n := householdSeats(t, db, userID); n != 0 {
		t.Errorf("%v has %d household seats, want none", userID, n)
	}
	for _, id := range liveFoods(t, db, userID) {
		if g := liveGroup(t, db, userID, id); g != "" {
			t.Errorf("%v's food %v is in group %s, want none", userID, id, g)
		}
	}
}

// kitchenOfTwo is Ana's and Ivo's household, of a tuna, a curry of Ana's
// chicken and kefir, and Ivo's bar, and Ivo's meal of his tuna.
func kitchenOfTwo(t *testing.T) (db *store.DB, ana, ivo uuid.UUID) {
	t.Helper()
	db, ana, ivo = withTwoUsers(t)
	saveFood(t, db, ana, tuna(116))
	saveRecipe(t, db, ana, curryOf(t, saveFood(t, db, ana, chickenBreast()), saveFood(t, db, ana, kefir())))
	saveFood(t, db, ivo, bar())
	join(t, db, ana, ivo)
	can, err := meal.FromFood(copyNamed(t, db, ivo, "Tuna u vlastitom soku"), meal.Quantity{Servings: 1, Serving: "can"}, "")
	if err != nil {
		t.Fatal(err)
	}
	createMeal(t, db, ivo, meal.Meal{ID: uuid.NewV7(), EatenAt: t0, Day: day.DateOf(t0), Client: meal.ClientMCP, Items: []meal.Item{can}})
	return db, ana, ivo
}

// A leave of a household of two dissolves it: each keeps every food and
// recipe as it was, theirs alone, the recipe of their own foods, and the
// meals of them; a save of one's reaches the other no more. The leave is
// the leaver's event, which names no one, and takes their invites away.
func TestLeaveAHouseholdOfTwo(t *testing.T) {
	t.Parallel()

	db, ana, ivo := kitchenOfTwo(t)
	eva := addEva(t, db)
	invite(t, db, ivo, eva)
	before := map[uuid.UUID]string{ana: kitchenDump(t, db, ana), ivo: kitchenDump(t, db, ivo)}

	leave(t, db, ivo)
	if es, err := db.Events(t.Context(), ivo, 1); err != nil || len(es) != 1 || es[0].Action != event.HouseholdLeave ||
		es[0].Entity != "household" || string(es[0].After) != `{"dissolved":true}` || es[0].Before != nil {
		t.Errorf("Ivo's latest event = %+v, %v; want household.leave of household, dissolved, naming no one", es, err)
	}
	alone(t, db, ivo)
	alone(t, db, ana)
	for u, k := range before {
		if got := kitchenDump(t, db, u); got != k {
			t.Errorf("%v's kitchen changed:\n%s\nwas\n%s", u, got, k)
		}
	}
	if n := countRows(t, db, "households"); n != 0 {
		t.Errorf("households = %d, want the household dissolved", n)
	}
	ownIngredients(t, db, ivo, recipeOf(t, db, ivo))
	ownIngredients(t, db, ana, recipeOf(t, db, ana))
	if ms, err := db.MealsOfDay(t.Context(), ivo, day.DateOf(t0)); err != nil || len(ms) != 1 {
		t.Errorf("Ivo's meals = %+v, %v; want his breakfast", ms, err)
	}
	if got, err := db.HouseholdInvitesTo(t.Context(), eva, t0); err != nil || len(got) != 0 {
		t.Errorf("Eva's invites = %+v, %v; want Ivo's gone with him", got, err)
	}

	apart(t, db, ana, ivo)
	if err := db.LeaveHousehold(t.Context(), ivo, testActor, t0); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("a leave of no household = %v, want ErrNotFound", err)
	}
}

// apart fails the test when a save of Ana's tuna reaches Ivo's, or one
// of his curry hers.
func apart(t *testing.T, db *store.DB, ana, ivo uuid.UUID) {
	t.Helper()
	tunaOf := copyNamed(t, db, ana, "Tuna u vlastitom soku")
	tunaOf.Nutrients.Kcal = 120
	saveFood(t, db, ana, tunaOf)
	if got := copyNamed(t, db, ivo, "Tuna u vlastitom soku"); got.Nutrients.Kcal != 116 {
		t.Errorf("Ivo's tuna after Ana's save has %v kcal, want his 116", got.Nutrients.Kcal)
	}
	ivos := recipeOf(t, db, ivo)
	ivos.Yield = 900
	saveRecipe(t, db, ivo, ivos)
	if got := recipeOf(t, db, ana); got.Yield != 1000 {
		t.Errorf("Ana's curry after Ivo's save weighs %v, want her 1000", got.Yield)
	}
}

// withThree is Ana's and Ivo's kitchenOfTwo, and Mia, with a kefir of
// her own, the third member.
func withThree(t *testing.T) (db *store.DB, ana, ivo, mia uuid.UUID) {
	t.Helper()
	db, ana, ivo = kitchenOfTwo(t)
	mia = addMia(t, db)
	saveFood(t, db, mia, kefir())
	join(t, db, ana, mia)
	return db, ana, ivo, mia
}

// A leave of a household of three keeps the two who stay one kitchen;
// the one who goes keeps theirs as it was, alone: a save of the others'
// reaches them no more, nor one of theirs the others.
func TestLeaveAHouseholdOfThree(t *testing.T) {
	t.Parallel()

	db, ana, ivo, mia := withThree(t)
	was := kitchenDump(t, db, mia)
	leave(t, db, mia)
	householdHolds(t, db, ana, ivo)
	alone(t, db, mia)
	if got := kitchenDump(t, db, mia); got != was {
		t.Errorf("Mia's kitchen changed:\n%s\nwas\n%s", got, was)
	}
	ownIngredients(t, db, mia, recipeOf(t, db, mia))
	if es, err := db.Events(t.Context(), mia, 1); err != nil || len(es) != 1 || string(es[0].After) != `{}` {
		t.Errorf("Mia's leave = %+v, %v; want it of a household that stays", es, err)
	}

	b := copyNamed(t, db, ana, bar().Name)
	b.Nutrients.Kcal = 370
	saveFood(t, db, ana, b)
	if i, m := copyNamed(t, db, ivo, b.Name), copyNamed(t, db, mia, b.Name); i.Nutrients.Kcal != 370 || m.Nutrients.Kcal != bar().Nutrients.Kcal {
		t.Errorf("after Ana's save, Ivo's bar has %v kcal, Mia's %v; want 370, and hers as it was", i.Nutrients.Kcal, m.Nutrients.Kcal)
	}
	k := copyNamed(t, db, mia, "Kefir")
	k.Nutrients.Kcal = 60
	saveFood(t, db, mia, k)
	for _, u := range []uuid.UUID{ana, ivo} {
		if got := copyNamed(t, db, u, "Kefir"); got.Nutrients.Kcal != kefir().Nutrients.Kcal {
			t.Errorf("%v's kefir after Mia's save has %v kcal, want it as it was", u, got.Nutrients.Kcal)
		}
	}
	householdHolds(t, db, ana, ivo)
}

// Undo refuses a leave, again and again, and never goes past it to the
// leaver's changes before; another member's undo of a change from before
// the leave puts back the copies of the household's members alone, not
// the leaver's.
func TestUndoAfterALeave(t *testing.T) {
	t.Parallel()

	db, ana, ivo, mia := withThree(t)
	b := copyNamed(t, db, ana, bar().Name)
	b.Nutrients.Kcal = 370
	saveFood(t, db, ana, b)
	k := copyNamed(t, db, mia, "Kefir")
	k.Nutrients.Kcal = 60
	saveFood(t, db, mia, k)
	leave(t, db, mia)

	for i := range 2 {
		if _, err := db.UndoLast(t.Context(), mia, testActor, t0.Add(time.Minute)); !isConflict(err, store.UndoHousehold) {
			t.Fatalf("Mia's undo #%d after the leave = %v, want UndoHousehold", i+1, err)
		}
	}
	request := uuid.NewV7()
	for i := range 2 {
		if _, _, err := db.Undo(t.Context(), mia, testActor, request, time.Now()); !isConflict(err, store.UndoHousehold) {
			t.Fatalf("Mia's undo #%d under a request after the leave = %v, want UndoHousehold", i+1, err)
		}
	}
	for _, u := range []uuid.UUID{ana, ivo, mia} {
		if got := copyNamed(t, db, u, "Kefir"); got.Nutrients.Kcal != 60 {
			t.Errorf("%v's kefir after Mia's undos has %v kcal, want her save's 60", u, got.Nutrients.Kcal)
		}
	}

	undoAt(t, db, ana, 2*time.Minute)
	for u, want := range map[uuid.UUID]float64{ana: bar().Nutrients.Kcal, ivo: bar().Nutrients.Kcal, mia: 370} {
		if got := copyNamed(t, db, u, b.Name); got.Nutrients.Kcal != want {
			t.Errorf("%v's bar after Ana's undo has %v kcal, want %v", u, got.Nutrients.Kcal, want)
		}
	}
}

// soupOf is userID's soup, a curry of other foods under another name,
// saved.
func soupOf(t *testing.T, db *store.DB, userID uuid.UUID) recipe.Recipe {
	t.Helper()
	r := curryOf(t, saveFood(t, db, userID, renamed(chickenBreast(), "Turkey")), saveFood(t, db, userID, renamed(kefir(), "Yoghurt")))
	r.Food.Name, r.Food.Aliases = "Soup", nil
	return saveRecipe(t, db, userID, r)
}

// soupMembers is the members of userID's soup, by id.
func soupMembers(t *testing.T, db *store.DB, userID uuid.UUID) []uuid.UUID {
	t.Helper()
	members, err := db.RecipeMembers(t.Context(), userID, copyNamed(t, db, userID, "Soup").ID)
	if err != nil {
		t.Fatalf("RecipeMembers: %v", err)
	}
	var out []uuid.UUID
	for _, m := range members {
		out = append(out, m.ID)
	}
	return sortedIDs(out)
}

// soupOfUser is userID's soup.
func soupOfUser(t *testing.T, db *store.DB, userID uuid.UUID) recipe.Recipe {
	t.Helper()
	r, err := db.RecipeByID(t.Context(), userID, copyNamed(t, db, userID, "Soup").ID)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// cookSoup saves userID's soup again, of yield.
func cookSoup(t *testing.T, db *store.DB, userID uuid.UUID, yield float64) {
	t.Helper()
	r := soupOfUser(t, db, userID)
	r.Yield = yield
	saveRecipe(t, db, userID, r)
}

// soupYield is the yield of userID's soup.
func soupYield(t *testing.T, db *store.DB, userID uuid.UUID) float64 {
	t.Helper()
	return soupOfUser(t, db, userID).Yield
}

// A recipe a user outside shared with a member is the household's too
// (story 403), the others in its group by the household's seats. The
// member it was shared with keeps it, with the user outside, when they
// leave; the others, in it by the household alone, keep it as the
// household's, a group of their own, so a save on either side stays
// there. A member who leaves it a household's recipe goes from its group,
// and the member it was shared with keeps it, with the household.
func TestLeaveARecipeSharedFromOutside(t *testing.T) {
	t.Parallel()

	t.Run("the member it was shared with leaves", func(t *testing.T) {
		t.Parallel()
		db, ana, ivo, mia := withThree(t)
		eva := addEva(t, db)
		share(t, db, eva, soupOf(t, db, eva).Food.ID, ivo)
		leave(t, db, ivo)
		if got := soupMembers(t, db, eva); !slices.Equal(got, sortedIDs([]uuid.UUID{eva, ivo})) {
			t.Errorf("Eva's soup's members = %v, want Eva and Ivo", got)
		}
		if got := soupMembers(t, db, ana); !slices.Equal(got, sortedIDs([]uuid.UUID{ana, mia})) {
			t.Errorf("Ana's soup's members = %v, want Ana and Mia", got)
		}
		householdHolds(t, db, ana, mia)
		if n := householdSeats(t, db, ivo); n != 0 {
			t.Errorf("Ivo has %d household seats, want none", n)
		}
		cookSoup(t, db, eva, 700)
		cookSoup(t, db, ana, 800)
		for u, want := range map[uuid.UUID]float64{eva: 700, ivo: 700, ana: 800, mia: 800} {
			if got := soupYield(t, db, u); got != want {
				t.Errorf("%v's soup weighs %v, want %v", u, got, want)
			}
		}
	})

	t.Run("another member leaves", func(t *testing.T) {
		t.Parallel()
		db, ana, ivo, mia := withThree(t)
		eva := addEva(t, db)
		share(t, db, eva, soupOf(t, db, eva).Food.ID, ivo)
		leave(t, db, ana)
		if got := soupMembers(t, db, eva); !slices.Equal(got, sortedIDs([]uuid.UUID{eva, ivo, mia})) {
			t.Errorf("Eva's soup's members = %v, want Eva, Ivo and Mia", got)
		}
		if got := soupMembers(t, db, ana); got != nil {
			t.Errorf("Ana's soup's members = %v, want hers alone", got)
		}
		alone(t, db, ana)
		householdHolds(t, db, ivo, mia)
		cookSoup(t, db, eva, 700)
		if got := soupYield(t, db, ana); got != 1000 {
			t.Errorf("Ana's soup after Eva's save weighs %v, want 1000 still", got)
		}
	})

	t.Run("the household is dissolved", func(t *testing.T) {
		t.Parallel()
		db, ana, ivo := kitchenOfTwo(t)
		eva := addEva(t, db)
		share(t, db, eva, soupOf(t, db, eva).Food.ID, ivo)
		leave(t, db, ivo)
		if got := soupMembers(t, db, eva); !slices.Equal(got, sortedIDs([]uuid.UUID{eva, ivo})) {
			t.Errorf("Eva's soup's members = %v, want Eva and Ivo", got)
		}
		alone(t, db, ana)
		cookSoup(t, db, ana, 800)
		if got := soupYield(t, db, ivo); got != 1000 {
			t.Errorf("Ivo's soup after Ana's save weighs %v, want 1000 still", got)
		}
	})
}

// share is userID's share of their recipe id with with.
func share(t *testing.T, db *store.DB, userID, id, with uuid.UUID) {
	t.Helper()
	if err := db.ShareRecipe(t.Context(), userID, testActor, id, with, t0); err != nil {
		t.Fatalf("ShareRecipe: %v", err)
	}
}

// A recipe shared again with a member who left it (with their household,
// when another member's share was stopped) is theirs by the share, not
// by the household: they keep it when they leave the household.
func TestShareAgainIsExplicit(t *testing.T) {
	t.Parallel()

	db, ana, ivo := kitchenOfTwo(t)
	eva := addEva(t, db)
	soup := soupOf(t, db, eva)
	share(t, db, eva, soup.Food.ID, ana)
	if err := db.UnshareRecipe(t.Context(), eva, testActor, soup.Food.ID, seatOf(t, db, eva, soup.Food.ID, ana), t0); err != nil {
		t.Fatal(err)
	}
	share(t, db, eva, soup.Food.ID, ivo)
	leave(t, db, ivo)
	if got := soupMembers(t, db, eva); !slices.Equal(got, sortedIDs([]uuid.UUID{eva, ivo})) {
		t.Errorf("Eva's soup's members = %v, want Eva and Ivo", got)
	}
}

// A member's account deleted leaves the household first: the others keep
// one kitchen, or each their own when one is left, everything as it was.
func TestDeleteAMember(t *testing.T) {
	t.Parallel()

	db, ana, ivo, mia := withThree(t)
	eva := addEva(t, db)
	invite(t, db, mia, eva)
	if _, err := db.DeleteUser(t.Context(), mia); err != nil {
		t.Fatal(err)
	}
	householdHolds(t, db, ana, ivo)
	if got, err := db.HouseholdInvitesTo(t.Context(), eva, t0); err != nil || len(got) != 0 {
		t.Errorf("Eva's invites = %+v, %v; want Mia's gone", got, err)
	}

	was := kitchenDump(t, db, ana)
	if _, err := db.DeleteUser(t.Context(), ivo); err != nil {
		t.Fatal(err)
	}
	alone(t, db, ana)
	if got := kitchenDump(t, db, ana); got != was {
		t.Errorf("Ana's kitchen changed:\n%s\nwas\n%s", got, was)
	}
	if n := countRows(t, db, "households"); n != 0 {
		t.Errorf("households = %d, want none", n)
	}
}

// An export holds the user's household, their own row of its members,
// their seats, and the invites they made and those made to them; no other
// member's row.
func TestExportAMember(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	eva, mia := addEva(t, db), addMia(t, db)
	saveFood(t, db, ana, bar())
	invite(t, db, mia, ivo)
	join(t, db, ana, ivo)
	invite(t, db, ivo, eva)
	invite(t, db, ana, mia)

	tables, err := db.ExportUser(t.Context(), ivo)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]store.ExportTable{}
	for _, tb := range tables {
		got[tb.Name] = tb
	}
	cell := func(table string, row int, col string) any {
		tb := got[table]
		return tb.Rows[row][slices.Index(tb.Columns, col)]
	}
	household := membersHousehold(t, db, ivo)
	if tb := got["households"]; len(tb.Rows) != 1 || cell("households", 0, "id") != household {
		t.Errorf("households = %+v, want Ivo's", tb)
	}
	if tb := got["household_members"]; len(tb.Rows) != 1 || cell("household_members", 0, "user_id") != ivo.String() {
		t.Errorf("household_members = %+v, want Ivo's row alone", tb)
	}
	var invites []string
	for i := range got["household_invites"].Rows {
		invites = append(invites, fmt.Sprint(cell("household_invites", i, "user_id"), ">", cell("household_invites", i, "invited_id")))
	}
	slices.Sort(invites)
	want := []string{ivo.String() + ">" + eva.String(), mia.String() + ">" + ivo.String()}
	slices.Sort(want)
	if !slices.Equal(invites, want) {
		t.Errorf("household_invites = %v, want Mia's to Ivo and his to Eva", invites)
	}
	if tb := got["food_shares"]; len(tb.Rows) != 1 || cell("food_shares", 0, "user_id") != ivo.String() {
		t.Errorf("food_shares = %+v, want Ivo's seat of the bar", tb)
	}
}

// membersHousehold is the id of userID's household.
func membersHousehold(t *testing.T, db *store.DB, userID uuid.UUID) string {
	t.Helper()
	var h string
	if err := db.Read().QueryRowContext(t.Context(), "SELECT household_id FROM household_members WHERE user_id = ?",
		userID.String()).Scan(&h); err != nil {
		t.Fatal(err)
	}
	return h
}

// A join and a leave are among the recent changes, but undo takes
// neither back, nor anything before: none is next until a change after.
func TestRecentChangesOfAHousehold(t *testing.T) {
	t.Parallel()

	db, ana, ivo := withTwoUsers(t)
	saveFood(t, db, ivo, kefir())
	join(t, db, ana, ivo)
	nextOf := func(want ...event.Action) {
		t.Helper()
		cs, err := db.RecentChanges(t.Context(), ivo, 10)
		if err != nil || len(cs) != len(want) {
			t.Fatalf("Ivo's changes = %+v, %v; want %v", cs, err, want)
		}
		for i, c := range cs {
			if c.Event.Action != want[i] || c.Next != (i == 0 && undoableAction(want[0])) {
				t.Errorf("change %d = %s, next %v; want %s", i, c.Event.Action, c.Next, want[i])
			}
		}
	}
	nextOf(event.HouseholdJoin, event.FoodSave)
	leave(t, db, ivo)
	nextOf(event.HouseholdLeave, event.HouseholdJoin, event.FoodSave)
	saveFood(t, db, ivo, chickenBreast())
	nextOf(event.FoodSave, event.HouseholdLeave, event.HouseholdJoin, event.FoodSave)
}

func undoableAction(a event.Action) bool {
	return a != event.HouseholdJoin && a != event.HouseholdLeave
}

// The statements docs/DESIGN.md gives an admin to take a member out by
// hand are household_leave.sql, and do what LeaveHousehold does, but its
// event: run on a copy of the database, they leave it as the leave does.
func TestAdminLeaveIsALeave(t *testing.T) {
	t.Parallel()

	db, ana, ivo, mia := withThree(t)
	eva := addEva(t, db)
	share(t, db, eva, soupOf(t, db, eva).Food.ID, ivo)
	invite(t, db, ivo, eva)
	twin := twinOf(t, db)

	leave(t, db, ivo)
	statements := adminLeave(t)
	err := twin.WriteTx(t.Context(), func(tx *sql.Tx) error {
		for _, s := range statements {
			var args []any
			if strings.Contains(s, ":user") {
				args = append(args, sql.Named("user", ivo.String()))
			}
			if _, err := tx.ExecContext(t.Context(), s, args...); err != nil {
				return fmt.Errorf("%s: %w", s, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if a, b := householdState(t, db, ana, ivo, mia, eva), householdState(t, twin, ana, ivo, mia, eva); a != b {
		t.Errorf("the admin's leave left\n%s\nthe leave\n%s", b, a)
	}
}

// adminLeave is the statements of the SQL block of docs/DESIGN.md that
// takes a member out, which must be household_leave.sql.
func adminLeave(t *testing.T) []string {
	t.Helper()
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "DESIGN.md"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.ReadFile("household_leave.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), "```sql\n"+string(file)+"```\n") {
		t.Fatal("docs/DESIGN.md does not give household_leave.sql as it is")
	}
	var out []string
	for s := range strings.SplitSeq(string(file), ";\n") {
		var code []string
		for line := range strings.SplitSeq(s, "\n") {
			if l := strings.TrimSpace(line); l != "" && !strings.HasPrefix(l, "--") {
				code = append(code, line)
			}
		}
		if len(code) > 0 {
			out = append(out, strings.Join(code, "\n"))
		}
	}
	return out
}

// twinOf is a copy of db, opened.
func twinOf(t *testing.T, db *store.DB) *store.DB {
	t.Helper()
	path, err := db.DailySnapshot(t.Context(), time.Now(), 1)
	if err != nil {
		t.Fatal(err)
	}
	twin, err := store.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = twin.Close() })
	return twin
}

// householdState is the kitchens of users, the households, their members
// and invites, and the share groups, each by its rows, not its id: what
// a leave changes, but its event.
func householdState(t *testing.T, db *store.DB, users ...uuid.UUID) string {
	t.Helper()
	var b strings.Builder
	for _, u := range users {
		b.WriteString(kitchenDump(t, db, u))
	}
	for _, q := range []string{
		"SELECT id || created_at FROM households ORDER BY id",
		"SELECT user_id || household_id || joined_at FROM household_members ORDER BY user_id",
		"SELECT id || user_id || invited_id FROM household_invites ORDER BY id",
	} {
		for _, s := range textRows(t, db, q) {
			fmt.Fprintln(&b, s)
		}
	}
	groups := map[string][]string{}
	rows := textRows(t, db, `SELECT share_id || ' ' || seat || ' ' || user_id || ' ' || food_id || ' ' || version || ' '
		|| coalesce(left_at, '-') || ' ' || household FROM food_shares`)
	for _, r := range rows {
		id, seat, _ := strings.Cut(r, " ")
		groups[id] = append(groups[id], seat)
	}
	var gs []string
	for _, g := range groups {
		slices.Sort(g)
		gs = append(gs, strings.Join(g, "; "))
	}
	slices.Sort(gs)
	for _, g := range gs {
		fmt.Fprintln(&b, g)
	}
	return b.String()
}

// eatSoup logs userID's meal of their soup.
func eatSoup(t *testing.T, db *store.DB, userID uuid.UUID) {
	t.Helper()
	bowl, err := meal.FromFood(copyNamed(t, db, userID, "Soup"), meal.Quantity{Amount: 300, Unit: "g"}, "")
	if err != nil {
		t.Fatal(err)
	}
	createMeal(t, db, userID, meal.Meal{ID: uuid.NewV7(), EatenAt: t0, Day: day.DateOf(t0), Client: meal.ClientMCP, Items: []meal.Item{bowl}})
}

// A stop of a share with a household, from before a member left it, is
// undone no more: it would give the one who left the recipe back with
// the household's, or the household with them, whichever of them it was
// shared with. They keep it as it was, left.
func TestUndoAStopAfterALeave(t *testing.T) {
	t.Parallel()

	for _, leaverIsRecipient := range []bool{false, true} {
		t.Run(fmt.Sprint("the recipient leaves: ", leaverIsRecipient), func(t *testing.T) {
			t.Parallel()
			db, ana, ivo := kitchenOfTwo(t)
			eva := addEva(t, db)
			soup := soupOf(t, db, eva)
			share(t, db, eva, soup.Food.ID, ana)
			eatSoup(t, db, ana)
			eatSoup(t, db, ivo)
			anas, ivos := copyNamed(t, db, ana, "Soup"), copyNamed(t, db, ivo, "Soup")
			if err := db.UnshareRecipe(t.Context(), eva, testActor, soup.Food.ID, seatOf(t, db, eva, soup.Food.ID, ana), t0); err != nil {
				t.Fatal(err)
			}
			leaver := ivo
			if leaverIsRecipient {
				leaver = ana
			}
			leave(t, db, leaver)
			if _, err := db.UndoLast(t.Context(), eva, testActor, t0.Add(time.Minute)); !isConflict(err, store.UndoChanged) {
				t.Errorf("Eva's undo of her stop after the leave = %v, want it changed since", err)
			}
			if !leftBy(t, db, ana, anas.ID) || !leftBy(t, db, ivo, ivos.ID) {
				t.Error("a soup is no longer left: want each theirs, for their meals")
			}
		})
	}
}

// An undo of a share again with a member who left it with their
// household, and of the stop before it, puts each seat back as it was:
// the member's the household's, which a leave takes, and the one it was
// shared with before an explicit share's, which a leave keeps.
func TestUndoAShareAgain(t *testing.T) {
	t.Parallel()

	db, ana, ivo := kitchenOfTwo(t)
	eva := addEva(t, db)
	soup := soupOf(t, db, eva)
	share(t, db, eva, soup.Food.ID, ana)
	eatSoup(t, db, ana)
	eatSoup(t, db, ivo)
	if err := db.UnshareRecipe(t.Context(), eva, testActor, soup.Food.ID, seatOf(t, db, eva, soup.Food.ID, ana), t0); err != nil {
		t.Fatal(err)
	}
	share(t, db, eva, soup.Food.ID, ivo)
	undoAt(t, db, eva, time.Minute)
	undoAt(t, db, eva, 2*time.Minute)
	leave(t, db, ivo)
	if got := soupMembers(t, db, eva); !slices.Equal(got, sortedIDs([]uuid.UUID{eva, ana})) {
		t.Errorf("Eva's soup's members = %v, want Eva and Ana: Ivo had it by the household", got)
	}
}
