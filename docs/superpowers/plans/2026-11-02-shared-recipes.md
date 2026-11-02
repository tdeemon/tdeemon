# Shared Recipes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
superpowers:subagent-driven-development (recommended) or
superpowers:executing-plans to implement this plan task-by-task. Steps use
checkbox (`- [ ]`) syntax for tracking.

**Goal:** A recipe can be shared with other users of the instance by
e-mail. It is then one recipe all of them have, change, find and log,
with each user's data otherwise their own.

**Architecture:** Every user a recipe is shared with keeps a recipe of
their own: their own `foods` row (source `recipe`), `recipes` row and
`recipe_items`, so search, `get_food`, a meal's item and its foreign key
stay per user, unchanged. A share group (`recipe_shares`) ties those
recipes together. A save of one writes the shared fields (the name, the
ingredients, the dish's weight, the portions, the pot) into every other
member's recipe in the same transaction, at the group's next version. An
ingredient may be a food of another member's (`recipe_items.food_user_id`);
its food, when that member deletes their account, goes, and the
ingredient keeps its values. Leaving a share keeps the user's recipe
for their meals but hides it from every pick.

**Tech Stack:** Go, SQLite (migrations in `internal/store/migrations`),
huma (`internal/api`), MCP (`internal/api/tools.go`), React + TanStack
Query (`web/`), Web Push (`internal/notify.Push`).

**Spec:** `docs/superpowers/specs/2026-11-02-shared-recipes-design.md`

## Global Constraints

- AGENTS.md rules hold for every task: tests fail without the change; a
  real SQLite file (`storetest.Open`); no personal data; code, comments,
  docs and commits in English; no AI attribution in commits or PRs;
  follow the linters; `make check` passes.
- Migrations only go forward: one new migration (`0032_recipe_shares.sql`),
  with the schema snapshot refreshed
  (`go test ./internal/store -run TestSchema -update`).
- The share operations are a web session's only: closed to API tokens
  (no `openToTokens`). MCP has no new tool.
- Notification of a share: Web Push alone (`notify.Push`), never Telegram.
- What crosses between users is a shared recipe's shared fields and,
  inside it, an ingredient's food's name, brand, basis, values and
  servings. Never a food's note, aliases or barcodes. Never a meal,
  metric, change, event or any other row.
- Per user, never shared: a recipe's note, its aliases, whether it is
  archived.
- After an API change: `go test ./internal/api -update` for
  `api/openapi.json`, then `(cd web/codegen && npm ci && npm run gen:api)`.
- Each new operation gets its case in `isolationCases`
  (`internal/api/isolation_test.go`) and `answerCases`
  (`internal/api/schema_test.go`). `newIsolationEnv` gets a recipe the
  two parties share.
- The web app: English and Croatian (`web/src/i18n`); a share, a stop and
  "Add to my foods" each said in a toast whose Undo takes it back.
- Stacked PRs, in order: store (Tasks 1–5), API and MCP (Tasks 6–8),
  web (Tasks 9–10), docs (Task 11). Each PR: `make check`, a Codex
  review, its findings fixed, then the PR.

## Review Focus

- A user leaves while another saves the recipe at the same moment: the
  save must not write into the leaver's recipe once they left. Task 3
  test `TestSaveSkipsWhoLeft`.
- Two members save one recipe one after the other. The second save
  wins whole; neither leaves the other's recipes half-written. Task 3
  test `TestSecondSaveOfASharedRecipeWins`.
- The recipient already has a food of the recipe's name. The share
  refuses it, naming the clash, rather than renaming anything of
  theirs. Task 4 test `TestShareRefusesANameTheRecipientHas`.
- An undo of an old save after another member's save. It must be
  refused (409), not roll back the other member's change. Task 5 test
  `TestUndoOfASharedSaveAfterAnothersIsRefused`.
- A member deletes their account. The others' recipes keep every
  ingredient with its values, and a later save by them is not refused.
  Task 3 test `TestDeletedMembersIngredientsStay`.

---

## PR 1: store

### Task 1: The schema and the domain

**Files:**

- Create: `internal/store/migrations/0032_recipe_shares.sql`
- Modify: `internal/domain/meal/meal.go` (`Item`),
  `internal/domain/event/event.go` (actions)
- Modify: `internal/store/schema.sql` snapshot (via `-update`)
- Test: `internal/store/migrate_recipe_shares_test.go`,
  `internal/domain/meal/meal_test.go`

**Interfaces:**

- Produces:
  - table
    `recipe_shares (share_id, user_id, food_id, version, left_at)`, `left_at`
    NULL while a member:
    `PRIMARY KEY (share_id, user_id)`, `UNIQUE (user_id, food_id)`,
    `FOREIGN KEY (user_id, food_id) REFERENCES foods (user_id, id) ON DELETE CASCADE`.
    `version` is the group's, the same in each live row, one more with
    each save, share or stop.
  - `recipe_items` rebuilt with `food_user_id TEXT`, the owner of the
    ingredient's food: the recipe's user for their own food, another
    member for theirs. `food_user_id` and `food_id` are both nullable.
    The one foreign key on them is
    `FOREIGN KEY (food_user_id, food_id) REFERENCES foods (user_id, id)`
    `ON DELETE SET NULL`,
    in place of `(user_id, food_id)`. A food that goes, because its
    owner deleted their account, leaves both NULL, and the ingredient
    keeps its values. The index is `recipe_items_food (food_user_id, food_id)`.
    The migration copies every row with `food_user_id = user_id`.
    `foodInUse` (`internal/store/recipes.go`) then asks
    `recipe_items WHERE food_user_id = ? AND food_id = ?`, so a food of
    the user's in a member's recipe counts as in use and is archived,
    not deleted, by an undo.
  - `events` rebuilt with the operations `'recipe.share'` and
    `'recipe.unshare'` added to its CHECK, as `0024_recipes.sql`
    rebuilt it.
  - `meal.Item.FoodUser uuid.UUID`: for a recipe's ingredient only,
    the user whose food `FoodID` is when it is not the recipe's user;
    `uuid.Nil()` otherwise. A meal never sets it, so `Item.sent()` and
    `SameAs` ignore it.
  - `event.RecipeShare = "recipe.share"`,
    `event.RecipeUnshare = "recipe.unshare"`.

- [ ] **Step 1:** Write `TestMigrationOfRecipeShares` (an `0031` database
  with a recipe of two ingredients, migrated). Assert: both rows are in
  `recipe_items` with `food_user_id` the recipe's user and the same
  values. Assert that deleting a food's owner (a user deleted) leaves
  another user's ingredient row with `food_id` NULL and its values. Assert
  `recipe_shares` exists and is empty. Assert an event of operation
  `recipe.share` inserts. Follow `migrate_food_reference_test.go`.
- [ ] **Step 2:** Run
      `go test ./internal/store -run TestMigrationOfRecipeShares`.
  Expected: FAIL (no migration).
- [ ] **Step 3:** Write the migration (the README's rules, `sqlfluff fix`)
  and the two constants and the `Item` field.
- [ ] **Step 4:** Run the migration test, then
      `go test ./internal/store -run TestSchema -update`
  and `go test ./internal/domain/... ./internal/store`. Expected: PASS.
- [ ] **Step 5:** Commit `feat(store): the schema of shared recipes`.

### Task 2: Reading a shared recipe

**Files:**

- Modify: `internal/store/recipes.go` (`recipeInTx`, `RecipesOf`),
  `internal/store/foods.go` (searches, `FoodByID`, `FoodByBarcode`, lists)
- Create: `internal/store/recipe_shares.go`
- Test: `internal/store/recipe_shares_test.go`

**Interfaces:**

- Produces:
  - `type RecipeMember struct { ID uuid.UUID; Name string; Email string }`
  - `RecipeMembers(ctx, userID, foodID uuid.UUID) ([]RecipeMember, error)`:
    the live members of the group of userID's recipe foodID, userID
    first. An unshared recipe gives `nil`. A recipe userID left, or
    none of theirs, gives ErrNotFound.
  - `recipe.Recipe.Items[i].FoodUser` read from `food_user_id`:
    `uuid.Nil()` when it is the recipe's user, so a recipe of one's own
    foods reads as before. A NULL `food_id` (the food gone) reads as
    `FoodID` Nil with the stored values.
  - `withdrawn`, a predicate on a food `f`: no `recipe_shares` row of
    `f`'s user and id with `left_at` set;
    a predicate every pick of a food holds to (FindFoods,
    FindArchivedFoods, the lists, FoodByID, FoodByKey, FoodByBarcode,
    LastUses unchanged). A food is never withdrawn from a meal's read.

- [ ] **Step 1:** Write `TestRecipeMembers` (a group of Ana and Ivo, made
  by inserting rows directly). Assert the order is Ana then Ivo. Assert
  Eva's call is ErrNotFound. Assert an unshared recipe gives nil.
  Write `TestWithdrawnRecipeIsNoPick`: a member row with `left_at` set.
  Assert FindFoods, FindArchivedFoods, FoodByID and the list leave it
  out. Assert a meal of it still reads with its values.
- [ ] **Step 2:** Run them. Expected: FAIL.
- [ ] **Step 3:** Implement `RecipeMembers` and the predicate, and read
  `food_user_id`.
- [ ] **Step 4:** Run `go test ./internal/store`. Expected: PASS.
- [ ] **Step 5:** Commit
      `feat(store): the members of a shared recipe, and a recipe left is no pick`.

### Task 3: Saving a shared recipe

**Files:**

- Modify: `internal/store/recipes.go` (`saveRecipeInTx`, `freshItems`,
  `checkCycle`, `writeRecipe`)
- Modify: `internal/store/recipe_shares.go`
- Test: `internal/store/recipe_shares_test.go`

**Interfaces:**

- Consumes: Task 1's schema and `Item.FoodUser`, Task 2's `RecipeMembers`.
- Produces: `SaveRecipe`/`CorrectRecipe` keep their signatures and behave
  as follows.
  - An ingredient's food may be the user's own (`FoodUser` Nil) or a
    food of another live member of this recipe's group (`FoodUser` set).
    Any other user's food is ErrInvalidRecipe ("no such food").
  - `freshItems` reads another member's food from that member's foods
    (`foodInTx(ctx, tx, item.FoodUser, item.FoodID)`). A food that is
    archived, gone (account deleted, `food_id` NULL) or of a member who
    left keeps the item's stored values and quantity, and is not
    refused.
  - After the saver's own recipe is written,
    `syncShared(ctx, tx, userID, foodID, made recipe.Recipe, now)`
    writes, for every other live member, their `foods` row's name (and
    `name_key`), the `recipes` row (yield, portions, pot) and the
    `recipe_items` (each ingredient's `food_user_id` set to the food's
    owner, the saver for the saver's own foods). It also writes the
    recalculated values (`recipe.Make` per member, the same numbers).
    Their note, aliases and archived stay. Then it bumps every live
    row's `version`. A member's name clash, where another food of theirs
    has the name, refuses the save, a `*FoodConflictError` naming that
    member's food by id only.
  - `checkCycle` walks `recipe_items` across users: the next step is
    `(coalesce(ri.food_user_id, ri.user_id), ri.food_id)`.
  - The event is the saver's (`recipe.save`), with `before`/`after` of
    the saver's recipe. Its JSON carries the group version it left, as
    `"share_version"` beside the recipe, for Task 5.

- [ ] **Step 1:** Write the tests:
  - `TestSaveOfASharedRecipeIsEveryones`: Ivo saves "Cooked again" with
    another yield. Assert Ana's recipe has the yield, the items and the
    values. Assert Ana's note and alias are unchanged.
  - `TestAnotherMembersIngredient`: Ivo adds his own food as an
    ingredient. Assert Ana's recipe names it with `FoodUser` Ivo.
    Assert Ana's own foods do not have it.
  - `TestSaveTakesAMembersCorrection`: Ana corrects her food, then Ivo
    saves. Assert both recipes take the new values.
  - `TestDeletedMembersIngredientsStay`: Ivo's account is deleted.
    Assert Ana's recipe keeps his ingredient's values. Assert her save
    succeeds.
  - `TestSaveSkipsWhoLeft`: Ivo left. Assert Ana's save leaves Ivo's
    recipe as it was.
  - `TestSecondSaveOfASharedRecipeWins`: two saves in turn. Assert both
    recipes equal the second.
  - `TestForeignFoodOutsideTheGroupIsRefused`: Eva's food id with
    `FoodUser` Eva. Assert ErrInvalidRecipe.
  - `TestCycleAcrossMembers`: Ana's recipe holds the shared recipe, and
    the shared recipe holds Ana's recipe by Ivo's save. Assert
    ErrRecipeCycle.
- [ ] **Step 2:** Run them. Expected: FAIL.
- [ ] **Step 3:** Implement as the interface says.
- [ ] **Step 4:** Run `go test ./internal/store`. Expected: PASS.
- [ ] **Step 5:** Commit
      `feat(store): a save of a shared recipe is every member's`.

### Task 4: Sharing and stopping

**Files:**

- Modify: `internal/store/recipe_shares.go`
- Test: `internal/store/recipe_shares_test.go`

**Interfaces:**

- Consumes: Tasks 1–3.
- Produces:
  - `ShareRecipe(ctx, userID, a event.Actor, foodID, withID, now) error`.
    userID must be a live member, or the owner of an unshared recipe.
    It refuses: ErrNotFound for no such recipe of theirs; `ErrShareSelf`
    for oneself; `ErrShareMember` for one who is a member already.
    Otherwise it makes the group when there is none (`share_id` a new
    UUIDv7, version 1). It gives withID a recipe of their own: a new
    food id, the shared fields, an empty note and aliases, archived
    false. A food of withID's that has the name refuses the share, a
    `*FoodConflictError` of ErrNameTaken naming the recipe by name only.
    A recipient who left before gets their old recipe back: `left_at`
    cleared, its shared fields written. It records userID's
    `recipe.share` event, entity `foodID`, after
    `{"user": withID, "food": <recipient's food id>, "share_version": n}`.
  - `UnshareRecipe(ctx, userID, a event.Actor, foodID, memberID, now) error`.
    userID must be a live member. memberID is a live member, userID too
    (leaving). It sets that member's `left_at`. A group of one live
    member left is dissolved: its rows are deleted, and the one left's
    recipe is an ordinary recipe again. It records userID's
    `recipe.unshare` event, entity `foodID`, before
    `{"user": memberID, "food": <member's food id>, "share_version": n}`.
  - `var ErrShareSelf, ErrShareMember error`.

- [ ] **Step 1:** Write the tests:
  - `TestShareRecipe`: assert the recipient's food (source recipe, no
    note) and their search finding it. Assert both members are listed.
    Assert the event.
  - `TestShareRefusesANameTheRecipientHas`: assert ErrNameTaken. Assert
    nothing is written.
  - `TestShareRefusesSelfAndMember`.
  - `TestAMemberSharesOn`: Ivo shares with Eva. Assert all three are
    listed.
  - `TestLeave`: Ivo leaves. Assert his meal of it still reads, and his
    search and FoodByID do not find it. Assert Ana's recipe stays.
  - `TestTheLastLeftHasAPlainRecipe`.
  - `TestShareAgainAfterLeaving`: assert Ivo's old food comes back.
    Assert his meals still name it.
- [ ] **Step 2:** Run them. Expected: FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test ./internal/store`. Expected: PASS.
- [ ] **Step 5:** Commit
      `feat(store): a recipe shared with another user, and stopped`.

### Task 5: Undo across users

**Files:**

- Modify: `internal/store/recipes.go` (`takeBackRecipe`), the undo dispatch in
  `internal/store/undo.go` (or where `takeBackRecipe` is called)
- Modify: `internal/store/recipe_shares.go`
- Test: `internal/store/recipe_shares_test.go`

**Interfaces:**

- Consumes: the events of Tasks 3 and 4, with `share_version`.
- Produces: undo rules. Each one, when it fails, is the existing
  `clash{UndoStuck}` 409, which the next undo goes past.
  - `recipe.save` of a shared recipe: refused when the group's version
    is not the event's `share_version`, since another member changed it
    since. Refused also when its user left. Otherwise the recipe is put
    back for the saver and every live member (`syncShared` with
    `before`), and the version is bumped.
  - `recipe.save` that created a recipe (no `before`): refused while
    the recipe is in a group.
  - `recipe.share`: takes the recipe from the member that event added,
    that member alone, as `UnshareRecipe` would. Those that member
    shared it with keep it. Refused when the version moved on since.
  - `recipe.unshare`: gives the member back, as a share again (`left_at`
    cleared, shared fields written; a group made anew if it was
    dissolved). Refused when the version moved on since, or when the
    member's food has gone.

- [ ] **Step 1:** Write the tests:
  - `TestUndoOfASharedSaveAfterAnothersIsRefused`.
  - `TestUndoOfASharedSaveIsEveryones`.
  - `TestUndoOfCreationOfASharedRecipeIsRefused`.
  - `TestUndoOfAShareTakesItFromThatMemberAlone`: Ana→Ivo, Ivo→Eva,
    then Ana undoes. Assert Eva keeps it.
  - `TestUndoOfALeaveGivesItBack`.
  - `TestUndoOfAnEditAfterLeavingIsRefused`.
- [ ] **Step 2:** Run them. Expected: FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test ./internal/store`, then `make check`. Expected:
      PASS.
- [ ] **Step 5:** Commit `feat(store): undo of a shared recipe's changes`. Run
      the
  Codex review, fix its findings, and open PR 1:
  `feat(store): recipes shared between users`.

## PR 2: API and MCP

### Task 6: The share operations

**Files:**

- Create: `internal/api/recipe_shares.go`
- Modify: `internal/api/recipes.go` (`recipeBody` gets `shared_with`),
  `internal/api/api.go` (routes)
- Test: `internal/api/recipe_shares_test.go`, `isolation_test.go`,
  `schema_test.go`

**Interfaces:**

- Consumes: `ShareRecipe`, `UnshareRecipe`, `RecipeMembers`, `notify.Push`.
- Produces:
  - `POST /api/v1/recipes/{id}/shares` `{"email": string}` returns 200
    with the recipe (`recipeBody` whole). Web session only. Errors: 404
    (no recipe of the user's); 422 for an address of no user, which
    says "no user of this instance has that address" (the instance's
    users know each other: the spec's open point, decided so); 409 for
    a member already, or a name the recipient has (naming no food of
    theirs, by its name only); 422 for oneself.
  - `DELETE /api/v1/recipes/{id}/shares/{user_id}` returns 204. Web
    session only. 404 for no such member.
  - `recipeBody.SharedWith []recipeMemberBody` with
    `json:"shared_with,omitempty" nullable:"false"`, where
    `recipeMemberBody{ID string; Name string}`: the other live members,
    by name. No e-mail: the user typed it, the others need not see it.
  - On a share: `notify.Push` to the recipient,
    a `push.Notification` whose title is the localised "Ana shared a
    recipe with you" (the sharer's name), body the recipe's name, URL
    `/foods/{the recipient's food id}`,
    in the recipient's locale. A push that fails is logged, never the
    share's failure.
  - The share and stop events read in `/api/v1/changes` as their action.

- [ ] **Step 1:** Write the tests:
  - `TestShareARecipe`: assert 200, `shared_with` [Zoe], and Zoe's
    search finds it. Assert the push is sent (the push test helper of
    `push_helpers_test.go`).
  - `TestShareRefuses`: each error and its status.
  - `TestStopSharing`: assert 204, and Zoe's search does not find it.
  - `TestLeaveARecipe`.
  - `TestShareIsClosedToTokens`: assert 403.
  - The isolation cases for both operations: another user's recipe id
    is 404 and changes nothing.
  - The answer cases.
- [ ] **Step 2:** Run `go test ./internal/api -run 'Share|Isolation|Answer'`.
  Expected: FAIL.
- [ ] **Step 3:** Implement and add the routes, then regenerate the spec
  and the web types.
- [ ] **Step 4:** Run `go test ./internal/api`. Expected: PASS.
- [ ] **Step 5:** Commit
      `feat(api): share a recipe with another user, and stop`.

### Task 7: A shared recipe's ingredients through the API and MCP

**Files:**

- Modify: `internal/api/recipes.go` (items in and out), `internal/api/meals.go`
  (`foodOf`)
- Modify: `internal/api/tools.go`, `internal/api/mcp_instructions.md`
- Test: `internal/api/recipe_shares_test.go`, `internal/api/mcp_test.go`,
  `internal/api/mcp_isolation_test.go`

**Interfaces:**

- Consumes: Task 3's save rules.
- Produces:
  - An ingredient in a recipe's answer carries `food_id` and, when the
    food is another member's, `"theirs": true`. It never carries a
    user id.
  - `upsert_recipe` and `POST /api/v1/recipes` take an ingredient's
    `food_id` that is the recipe's own ingredient of another member's
    food. The API resolves it to `FoodUser` from the stored recipe.
    Any other user's food id stays "no such food" (422).
  - `get_food` of a shared recipe gives its ingredients as above, and
    `changed` for another member's food as for one's own.
  - The descriptions of `find_foods`, `get_food` and `upsert_recipe`,
    and the instructions, say in a sentence that a recipe may be
    shared, that a save is everyone's, and that its ingredients of
    another's food are kept by their `food_id`. The budget
    (`TestMCPFitsItsBudget`) is raised with the reason.
  - `mcpIsolationCases`: a tool call with another user's food as an
    ingredient of a recipe not shared is refused.

- [ ] **Step 1:** Write the tests:
  - `TestSaveKeepsAnothersIngredient`: Zoe saves with Ana's ingredient's
    `food_id`. Assert 200 and Ana's ingredient kept.
  - `TestSaveRefusesAForeignFood`.
  - The MCP isolation case.
- [ ] **Step 2:** Run them. Expected: FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `go test ./internal/api`. Expected: PASS.
- [ ] **Step 5:** Commit
      `feat(api): a shared recipe's ingredients of another member's foods`.

### Task 8: An ingredient added to one's foods

**Files:**

- Modify: `internal/api/recipe_shares.go`, `internal/store/recipe_shares.go`
- Test: `internal/api/recipe_shares_test.go`

**Interfaces:**

- Produces:
  - `IngredientFood(ctx, userID, recipeID uuid.UUID, position int) (food.Food, error)`:
    the food of ingredient `position` of userID's recipe when it is
    another member's: its name, brand, basis, nutrients, portion and
    servings, source as it is, no note, aliases or barcodes, and a new
    id. When the food is gone, it is made of the ingredient's stored
    values per 100 g or per portion, without servings. ErrNotFound for
    no such ingredient, or one of userID's own foods.
  - `POST /api/v1/recipes/{id}/ingredients/{position}/food` returns 201
    with the new food. It is a `CreateFood` of the user's: a name and
    brand they have is 409, naming their food. Web session only. Its
    event is the user's `food.save`, so undo takes it back.

- [ ] **Step 1:** Write `TestAddAnIngredientToMyFoods`. Assert 201, the
  values and servings, and no note, aliases or barcodes. Assert the
  recipe is unchanged. Assert undo takes the food back. Assert one's own
  ingredient gives 404. Assert a name one has gives 409. Add the
  isolation and answer cases.
- [ ] **Step 2:** Run it. Expected: FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `make check`. Expected: PASS.
- [ ] **Step 5:** Commit
      `feat(api): another member's ingredient added to one's foods`.
  Run the Codex review and fix its findings. Open PR 2,
  `feat(api): shared recipes in the API and MCP`, stacked on PR 1. Run
  the MCP eval once, after asking the maintainer.

## PR 3: web

### Task 9: Sharing on a recipe's page

**Files:**

- Create: `web/src/components/recipes/recipe-sharing.tsx`,
  `web/src/api/recipe-shares.ts`
- Modify: `web/src/components/recipes/recipe-detail.tsx`, `web/src/i18n/en.ts`,
  `web/src/i18n/hr.ts`, `web/src/lib/toast` (as other changes use it)
- Test: `web/src/components/recipes/recipe-sharing.test.tsx`

**Interfaces:**

- Consumes: Task 6's operations, `shared_with`.
- Produces:
  - `useShareRecipe(id)` and `useStopSharing(id)` invalidate the
    recipe, the foods' searches and lists.
  - On a recipe's page: "Shared with" with the members, each with
    Remove, and the user's own "Leave"; and "Share", a sheet of one
    e-mail field and a button.
  - Each change is said in a toast whose Undo names its change
    (`last: { action: 'recipe.share' | 'recipe.unshare', entity: <food id> }`).
  - The server's error is said in its words, at the field.

- [ ] **Step 1:** Write the tests:
  - A share sends the address and lists the member.
  - An error is said at the field.
  - Remove and Leave send the delete. After Leave, the app goes back to
    the foods.
  - Both languages.
- [ ] **Step 2:** Run
      `cd web && npx vitest run src/components/recipes/recipe-sharing.test.tsx`.
  Expected: FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run the web tests. Expected: PASS.
- [ ] **Step 5:** Commit `feat(web): share a recipe from its page`.

### Task 10: Another member's ingredients

**Files:**

- Modify: `web/src/components/recipes/ingredient-row.tsx`, `recipe-editor.tsx`,
  the i18n files
- Test: `web/src/pages/recipe.test.tsx`

**Interfaces:**

- Consumes: Task 7's `theirs`, Task 8's operation.
- Produces:
  - An ingredient that is `theirs` reads as the others do. Its row
    offers "Add to my foods", which says the new food in a toast with
    Undo. The editor keeps it by its `food_id` in a save. Its amount may
    be changed; its food cannot be opened as one's own.
  - The e2e test (`web/e2e/recipes.spec.ts`, or a new
    `shared-recipe.spec.ts`): two users, a share, the recipient logs a
    portion. Checked on a phone and a desktop, in both themes, with axe.

- [ ] **Step 1:** Write the tests:
  - A `theirs` ingredient shows "Add to my foods". It posts and says the
    toast.
  - A save keeps its `food_id`.
  - The e2e path.
- [ ] **Step 2:** Run them. Expected: FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Run `make check` and `make e2e`. Expected: PASS.
- [ ] **Step 5:** Commit
      `feat(web): another member's ingredients in a shared recipe`.
  Run the Codex review and fix its findings. Open PR 3, stacked on PR 2.

## PR 4: docs

### Task 11: The exception, written down

**Files:**

- Modify: `docs/DESIGN.md` (the "Recipes" paragraph, the table, §8 if undo is
  described there), `docs/conventions.md` (the isolation rule and its one
  exception), `AGENTS.md` (the project's list and "One user's data stays
  theirs"), `README.md` (the recipes' part, if it lists what the app does)

- [ ] **Step 1:** Write what the spec decided, as the docs' style says
  it. Cover: one recipe each member has; shared and own fields; the
  ingredients of another's; leaving; account deletion; undo; Web Push.
  Also cover the isolation exception and its tests.
- [ ] **Step 2:** Run `make check` (the docs' lint). Expected: PASS.
- [ ] **Step 3:** Commit `docs: shared recipes`, then open PR 4. Merge the
  stack in order once CI is green, as the maintainer allows.

## As built (PR 1)

What the store's tasks changed of the plan above, as their tests found:

- **Seats.** A member is named by their seat in the share (`recipe_shares.seat`,
  `UNIQUE (share_id, seat)`), not by their user's or food's id: the
  events of a share are the sharer's, and their export would hold the
  other's ids. `UnshareRecipe` takes a seat; `RecipeMember` has `Seat`.
  The API names members by seat too (Task 6:
  `DELETE /api/v1/recipes/{id}/shares/{seat}`,
  `shared_with` of seats and names).
- **No dissolving.** A share left with one live member keeps its rows:
  that member's recipe is an ordinary one (`RecipeMembers` is nil), and
  a share again gives one who left their old recipe back.
- **A deleted member's ingredients are adopted.** `DeleteUser` gives
  every other member a food of their own of each ingredient of the
  deleted user's food (`adoptIngredients`), so the ingredient stays a
  food they can change; the foreign key's `SET NULL` is the fallback.
- **The recipes row is written in place** (`INSERT … ON CONFLICT`), as
  deleting it took the share's rows along.
- **Isolation.** `newIsolationEnv` has the owner share a recipe with
  the other: the other's deletion then takes nothing of the owner's.
- **Frozen ingredients.** An ingredient of a member who left, or of a
  food archived in a shared recipe, or a recipe the user left, keeps the
  values the recipe has (`ingredientSource`, `frozenFood`, with the
  serving its quantity names); adoption copies those values, not the
  food as it came to be since. `IngredientFoods` gives the API each
  ingredient's food as a save reads it.
- **Undo across members.** A recipe save's event records the share's
  version (`recipeState.ShareVersion`), so a save that brings an earlier
  value back after another member's still stops the undo; an undo puts
  the version back (`setShareVersion`), so one's own undos go on.
- **A recipe left is no pick by name.** `FoodByKey` hides it, a new food
  of its name is refused (`ErrNameTaken`), a save of it is refused, and
  `RecentMeals` drops its meals.
- **Seats never repeat**: a new one is past every seat and version.
