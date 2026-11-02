# Shared recipes: design

Story 403, decided with the maintainer on 2 November 2026.

## Intent

Two users of one instance who cook together (a couple: one pot of
bolognese, both eat from it) should not each enter the same recipe. A
recipe can be shared with another user of the instance: from then on it
is one recipe both of them have, find and log as their own, and either
can change. The same pot means the same numbers: when either cooks it
again and weighs the pot, the new values per 100 g are both users'.

Nobody needs it yet: it is ready for when a second person of the house
uses tdeemon.

## Goals

- A recipe is shared with another user by their e-mail address, from
  the web app.
- A shared recipe is one recipe, not a copy per user: its ingredients,
  the dish's weight, the portions and the empty pot's weight are the
  same for every user it is shared with, and a change by any of them is
  everyone's from then on.
- Each user it is shared with finds it and logs it as any food of
  theirs, in the web app and through MCP; a meal keeps the values it
  was logged with, as every meal does.
- Every user it is shared with is equal: each may change it (web and
  MCP), share it with another user, and stop sharing it with anyone,
  themselves included.
- An ingredient a user lacks can be added to their own foods from the
  recipe, in a tap.

## Non-goals

- A household: a lasting link of two accounts whose recipes are all
  shared, chosen from a list. A later story; the e-mail address is
  enough now.
- Sharing anything but recipes (foods, meals, the plan).
- Tracking what is left of a pot (already "for later" in DESIGN.md).
- Sharing through MCP: an AI has no one to share with. MCP finds, logs
  and changes a shared recipe, never shares or stops sharing one.

## Anti-goals

- Anything of a user's other than the recipes they shared leaking to
  another: their foods (beyond an ingredient's name, amount and values
  inside a shared recipe), meals, metrics, plan, notes, changes.
- A user's past meals changing because someone else changed or stopped
  sharing a recipe.
- A user editing another user's food: an ingredient's food stays its
  owner's; the other sees it only through the recipe.
- The recipient having to accept: a share is there at once.

## Behaviour

### Sharing

- In the web app, a recipe's page offers to share it: the other user's
  e-mail address, then the share. The recipe appears among the
  recipient's foods (and recipes) at once, with no accepting, and they
  get a notification by Web Push alone (not Telegram) that it was shared
  with them, by whom.
- A recipe's page lists who it is shared with; any of them may add
  another user, or stop sharing it with any of them, themselves too
  ("leave").
- An address of no user, or of the user themselves, is refused, saying
  so. (Whether the refusal may reveal that an address has an account on
  the instance: see "Left to the builder".)

### Using and changing it

- Each user it is shared with finds it in a search, logs it in a
  portion, a share of the dish or grams, sees its page and cooks it
  again, as a recipe of their own; the AI does the same through
  `find_foods`, `get_food`, `log_meal`, `upsert_recipe`.
- A change by any of them (its ingredients, "Cooked again", the
  portions, the name) is the recipe's, so every user it is shared with
  has it from then on. History stays: logged meals keep their values.
- Ingredients: whoever changes the recipe adds ingredients from their
  own foods, the catalog or as a new food, which becomes theirs. The
  others see such an ingredient as its name, brand, amount and values
  inside the recipe, never among their own foods.
- An ingredient a user lacks offers "Add to my foods": a copy of its
  food (its name, brand, values, servings, barcodes; not its note or
  aliases) becomes theirs, as a catalog food's copy does, recorded as a
  change of theirs that undo takes back. The recipe does not change.
- An ingredient's food can be corrected only by its owner, in their
  foods. A save of the recipe after that takes the corrected values, as
  a save of a recipe does today.

### Stopping and leaving

- Stopping to share a recipe with a user (or the user leaving it): they
  can no longer pick it for a new meal, in any search, archived or not,
  nor see its page; their meals of it keep their values and still read
  as the recipe. The others keep it as it is.
- A user who deletes their account: the recipe stays with the users it
  was shared with, whole, ingredients of the deleted user's foods
  included as they are in the recipe (name, amount, values).
- A recipe shared with no one any more is an ordinary recipe of the one
  user left.

### Undo

Each change goes into the event log of the user who made it, as today,
so that user's undo takes it back: a change of the recipe, a share, a
stop. Undoing a change of a shared recipe after another user changed it
since is refused with a 409, as an undo of anything changed since is.

## Constraints

- "One user's data stays theirs" (AGENTS.md, docs/conventions.md) gets
  its first explicit exception: a recipe, and only a recipe, that its
  users shared. The exception is written down in docs/conventions.md
  and DESIGN.md, and tested. The isolation tests (`isolationCases`,
  `mcpIsolationCases`, `newIsolationEnv`) gain a shared recipe between
  the two parties and prove that only it, and nothing else of either
  party's, crosses: no food beyond an ingredient's fields inside the
  recipe, no meal, no change, no event.
- A meal item still names a food of the user who logged it (the store
  and the schema hold to that). How a shared recipe is a food of each
  of its users is the builder's, within that rule.
- Migrations only go forward; the existing recipes stay as they are,
  unshared.
- The API: new operations for sharing (a web session only, closed to
  tokens, as settings are) with their cases in `isolationCases` and
  `answerCases`; `api/openapi.json` and the web app's types
  regenerated.
- MCP: no new tool. The descriptions of `find_foods`, `get_food` and
  `upsert_recipe` say a recipe may be shared, and what that changes for
  a save; the budget (`TestMCPFitsItsBudget`) is kept or raised with a
  reason, and the eval run once after.
- The web app: English and Croatian; the access checks (axe, a finger's
  targets, focus) on a phone and a desktop; a share and a stop each
  said in a toast whose Undo takes it back.
- No personal data in the repo: test users are made up.

## Decided by the builder (my calls, to check)

- The notification of a share goes to the recipient's devices by Web
  Push alone (the maintainer's call: no Telegram for it); the app shows
  nothing more of it.
- An ingredient copied by "Add to my foods" keeps no link to the food
  it came from.
- Renaming a shared recipe renames it for all of its users.
- **What is shared, what is each user's.** Shared: the name, the
  ingredients, the dish's weight, the portions, the empty pot's weight.
  Each user's own: the note and the aliases (a user's own words, as
  DESIGN.md keeps them out of the catalog), and whether it is archived
  for them. Archiving a shared recipe hides it from that user's search
  alone, as any archived food; leaving it is the way to be rid of it.
- **A save and the others' ingredients.** A save of a shared recipe by
  any of its users takes every ingredient's values from its food as it
  is now, whoever's the food is ("the same pot, the same numbers"), so
  an owner's correction of a food reaches the recipe with its next save
  by anyone. An ingredient's food that is archived, or whose owner left
  the recipe or deleted their account, keeps the values the recipe
  holds; the save is never refused for it. Through MCP, a save takes
  the ids of the recipe's own ingredients' foods (the ones `get_food`
  shows), whoever's they are, and no other food of another user's.
- **What crosses with an ingredient.** Inside a shared recipe, the
  other users see an ingredient's food's name, brand, basis, values
  and servings, enough to give its quantity in a serving and to copy
  it, but not its note, aliases or barcodes. "Add to my foods" copies
  just those, so the new food has no barcode.
- **Undo across users.** An undo never takes a recipe from a user that
  the undone event did not give it to: undoing a recipe's creation is
  refused (409) while it is shared with anyone; undoing A's share with
  B takes it from B alone, and those B shared it with keep it. An undo
  of a change of a recipe that its user no longer has (they left, or
  were removed) is refused (409), and the next undo goes past it.
- **Recipes in recipes.** A recipe inside a shared recipe is an
  ingredient like any food: the others see it as one (its name and
  values), never as a recipe of theirs. A shared recipe may be an
  ingredient of a user's own recipe; a user who loses the shared one
  keeps it in their own recipe with the values it holds, as with a food
  archived. The check that a recipe is not its own ingredient at any
  depth runs across users, through the shared recipes.

## Left to the builder

- How the store models one recipe several users have (a recipe of no
  single owner and a food of it per user, or the owner's recipe and a
  share table, or another), as long as the behaviour, the meal item's
  rule and the isolation tests above hold.
- How a user's name clash is handled: a shared recipe whose name a food
  of the recipient's has already.
- Whether a refused address says "no such user" or answers alike for
  any address (the instance's users are few and known to each other).
- The order of the pull requests (store, API and MCP, web, docs), as
  the earlier stories split them.
