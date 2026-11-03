# Household: design

Story 408, planned with the maintainer on 2 November 2026. It builds on
the shared recipes of story 403
([2026-11-02-shared-recipes-design.md](2026-11-02-shared-recipes-design.md)).

## Intent

Two (or more) users of one instance who keep one kitchen, a couple
cooking and shopping together, link their accounts once into a
household. From then on they have one set of foods and recipes: a food
either of them enters, corrects or scans is both of theirs, and so is
every recipe. What each of them eats, weighs and plans stays theirs.

## Goals

- A user invites another user of the instance into their household by
  e-mail address, from the web app; the invited user accepts or
  declines. Accepting links the accounts.
- At the link, the two sets of foods and recipes merge into one: a food
  both have is one food, with the inviting side's values; a food only
  one side has comes to the other.
- From then on every food and every recipe of a member is every
  member's: a new one, a change, a barcode added, a recipe cooked again.
- Each member logs, finds, scans and changes them as their own, in the
  web app and through MCP; a meal keeps the values it was logged with.
- A household has two or more members, all equal: any member invites,
  and anyone may leave. No member removes another.
- Leaving keeps everything: the one who goes keeps
  their foods and recipes as ordinary ones of their own, as they were at
  that moment; the others keep theirs.

## Non-goals

- Sharing meals, metrics, the plan, the profile, notes for a coach,
  devices, tokens or changes. A household shares a kitchen, not a diet.
- Usage shared: how often a food is eaten, recent meals and the
  ordering of a search stay each member's, from their own meals.
- More than one household per user.
- Linking through MCP or the API tokens: an AI has no one to invite.
- Tracking what is left of a pot (DESIGN.md "for later").

## Anti-goals

- A member's past meals changing because another changed a food.
- Anything of a member's other than foods and recipes reaching another.
- A merge that loses a user's meal: every food a meal names stays that
  user's row, under its id.
- A link without the invited user's consent.

## Behaviour

### Inviting and accepting

- Settings has a Household row. Without a household it offers to invite
  someone by e-mail address; in one, it lists the members and the
  invites waiting, and offers to invite or leave.
- An invite goes to an existing user of the instance; the invited user
  gets a Web Push notification (by whom), and the app shows the invite
  on the Household page (the notification's tap opens it), and a line
  on the day until it is answered.
- An invite expires after 7 days. The inviting member may revoke it; the
  invited user may decline it. One invite waits per pair.
- Refused, saying so: an address of no user or of oneself, a user
  already in this household, a user who is in another household (they
  leave theirs first), a user with an invite from this household
  waiting.
- Before accepting, the invited user sees what the merge does, from the
  same computation as the merge itself (a dry run): how many foods and
  recipes become one, how many come to them, how many go to the
  household, and each one that is renamed or loses a barcode, by name.
  Accepting does what it showed, or is refused with a 409 when the
  household's foods changed since (the page offers the new preview).
- The inviting side is the household (any of its members invited);
  with no household yet, the inviter alone, and the accept makes the
  household of the two.
- On accept, the members get a Web Push notification that the user
  joined.

### The merge

The inviting side's foods (H, the household's) and the invited user's
(U) are matched; then each match becomes one food, and each food
without a match is copied to the side that lacks it.

1. **Matching.** Two foods match when any of these hold, checked in
   this order: the same name and brand (food.Key, as a user's foods are
   kept unique), the same barcode, the same catalog row (`catalog_id`).
   A food matches at most one food of the other side; an earlier rule
   wins over a later one, and among several at one rule the most
   recently saved one wins.
2. **The inviting side wins.** A matched pair becomes one food with H's
   shared fields: its name, brand, basis, values, servings, portion and
   source. U's row keeps its id, so U's meals and recipes still name
   it; it takes H's fields. U keeps its note, aliases and archived.
   Barcodes are the union of both, so none is lost, except one another
   food of the merged set already has (it stays there).
3. **The other side gets what it lacks.** A food of H's with no match
   gets a new row for U, a food of U's with no match a new row for each
   member of H, with its shared fields and no note or aliases.
4. **Kinds.** A recipe and a plain food never merge: a pair of
   different kinds (a match by name only) is no match, and U's one is
   renamed with a suffix (" (2)", the lowest free) and shared as its
   own. Two recipes merge as two foods do: H's ingredients, dish's
   weight, portions and pot; each member's recipe names each
   ingredient's food by that member's own copy of it.
5. **Recipes shared outside.** A recipe of U's in a share group with
   users outside the household (story 403) keeps its group: the
   household's members are added to it, and if it matched a recipe of
   H's, U's copy leaves that outside group (its other members keep the
   recipe) and joins H's.
6. **Collisions after renaming or a barcode.** Whatever the rules
   above leave clashing (a name, a barcode) is resolved by renaming U's
   one, or dropping the barcode from U's one, and is shown in the
   preview.

The whole merge is one transaction.

### Living in a household

- **One kitchen.** Every food of a member has a copy in each member's
  foods, all copies tied in a share group (generalising 403's
  `recipe_shares` to every food). A meal item names the copy of the
  user who logged it, as the store and the schema require.
- **What is shared, what is each member's.** Shared: the name, brand,
  basis, values, servings, portion, source, barcodes, and for a recipe
  its ingredients, dish's weight, portions and pot. Each member's: the
  note, the aliases, and archived. Barcodes are shared here, unlike in
  403's recipe sharing, because a household scans the same products.
- **Every path that makes or changes a food reaches the others in the
  same transaction:** a food created or corrected (web, MCP, API), a
  new food inside a meal (`log_meal`, `update_meal`), a catalog food
  copied on use, a barcode linked to a food on a log, an Open Food
  Facts product taken, "Add to my foods", a recipe saved or cooked
  again, a recipe shared with a member from outside (403) or by an
  outsider with a member (the other members join its group), an undo.
- **Search and a barcode** find the member's own copies only, as
  today: no lookup crosses users.
- **Equal members.** Any member changes any food or recipe; the latest
  save wins, as a user's own saves do.

### Leaving

- A member leaves, after confirming; no member removes another. The
  one who goes keeps every food and recipe they have as an ordinary
  one of theirs; their seats in the household's share groups go, and
  their ingredients name their own copies. A recipe group they were in
  by an explicit share (403) keeps them.
- A household left with one member is dissolved; that member keeps
  everything as ordinary foods and recipes.
- A member who deletes their account leaves the household first; the
  others keep everything.
- The members left get a Web Push notification that someone left.
- An admin who must take a member out does it in the database, by
  hand: the same as a leave (the member's row in the household and
  their household seats in `food_shares` go, a household of one is
  dissolved). DESIGN.md gives the statements. There is no operation
  for it in the app, the API or the CLI.

### Undo

- Each change goes into the event log of the member who made it, as
  today. Undoing a save of a household food takes it back for every
  member, refused with a 409 if any member saved it since (the group's
  version), as 403 does for recipes.
- Joining and leaving are recorded (`household.join`,
  `household.leave`), and are not undone: undo refuses them with a 409
  saying to leave, or invite again, and an undo never goes past one to
  the changes before it (it would bring back a state from before the
  merge into shared foods).
- Inviting, revoking and declining are not events; they change no data
  of a user's.

## Constraints

- "One user's data stays theirs" (AGENTS.md, docs/conventions.md) widens
  its exception from "a recipe shared between its users" to "the foods
  and recipes of a household's members". docs/conventions.md and
  DESIGN.md say so; the isolation tests gain a household between the
  two parties, and prove that its foods and recipes, the members' names
  and e-mail addresses, and nothing else cross: no meal, metric, plan,
  note, change, event, device or token. A third user outside the
  household sees nothing of it.
- Migrations only go forward: `recipe_shares` becomes `food_shares`
  (any food, the seat marked as the household's or an explicit
  share's), with new tables for households, their members and invites.
  Existing data stays as it is; no one is in a household.
- The API: new operations for the household (a web session only, closed
  to tokens, as settings are), with their cases in `isolationCases` and
  `answerCases`; `api/openapi.json` and the web app's types regenerated.
- MCP: no new tool. The descriptions of `upsert_food`, `upsert_recipe`
  and `get_food` (and the server's instructions, if needed) say a change
  reaches the household; the budget (`TestMCPFitsItsBudget`) is kept or
  raised with a reason, and the eval run once after, with a case of a
  household's food corrected.
- The export holds the user's household, their own membership of it
  (the other members' rows are theirs), their seats, and the invites
  into a household they made or got; the account's delete leaves the
  household first.
- The web app: English and Croatian; axe, a finger's targets and the
  focus on a phone and a desktop; the merge's preview readable on a
  phone; a leave asks before.
- No personal data in the repo: test users are made up.

## Decided by the builder (to check)

- Matching by name and brand, then barcode, then catalog row; the
  inviting side wins (the maintainer's rule), barcodes merged as a
  union.
- A recipe and a plain food of one name do not merge; the invited
  user's one is renamed.
- The note, aliases and archived stay each member's; barcodes are
  shared.
- Joining and leaving are not undone, and undo stops at them.
- A preview before accepting, from the merge's own dry run.
- No member removes another (the maintainer's call): only a leave; an
  admin may take a member out in the database by hand.
- The household's changes are not shown in the other members' recent
  changes; a member sees a food as it is now.

## Order of work

1. Store: `food_shares` (from `recipe_shares`) and the household tables,
   recipe sharing moved onto them with no change in behaviour.
2. Store: every food write of a member mirrored to the household, and
   its undo.
3. Store: invites and the merge, with its dry run.
4. Store: leaving, an account's delete and export; undo's barrier.
5. API and notifications.
6. MCP descriptions, budget and eval.
7. Web: the Household page, the invite, the preview and accept, leaving.
8. Docs: DESIGN.md, docs/conventions.md, README, AGENTS.md.
