# AGENTS.md

Instructions for AI coding agents working on tdeemon. Humans follow the
same rules; the full versions are in
[docs/conventions.md](docs/conventions.md), and the design is in
[docs/DESIGN.md](docs/DESIGN.md).

## The project

tdeemon is a self-hosted nutrition tracker with an adaptive TDEE: one Go
binary with SQLite and a React web app inside, an MCP server and a REST
API. It is early: the skeleton, the database layer, users, sign-in
(password, sessions, OpenID Connect), API tokens with scopes, foods
(and the instance's catalog of the admins' declared ones, which every user's
search finds after their own, and a meal copies),
meals, recipes (shared with other users too, by address), households
(users who keep one kitchen, every food and recipe of a member each
member's, joined by an invite after a preview of the merge, left by a
member alone), metrics and
the plan (profile, phases, targets, limits) in the
domain and the database with an event log and undo, the proposals of
changes of the plan and their life in the domain and the database, the
API for the admin's users (an invite's link that makes one account,
their list, a user deactivated, active again
or of another role, never the last active admin), an account's export
(a ZIP of every table of the user's, in JSON and CSV, but the secrets)
and its delete with all its data (by its user, the admin or the CLI,
never the last admin), the profile, the limits, the autopilot, the
settings, the notes for a coach,
the devices subscribed to notifications by Web Push (the key, a
subscription, their list, a delete and a test), the morning summary and
the watchdog's alarms by push to them, the morning summary of a date
whole and in a line, the list of foods, a
new food, a new recipe, the recent meals, the history and the plan, the user's own
change of the plan (a proposal applied at once), calibration
(the measured TDEE and the verdict on a phase), the autopilot's
weekly review decided in the domain and kept in the database (its
settings, its steps and proposals, which undo leaves, and its reviews),
run each week by the scheduler with its message to Telegram and by push
and its line in the morning summary, its settings in the API,
the tool registry with
`get_profile`, `get_day`, `get_summary`, `get_history`, `get_plan`,
`get_coaching_guide`,
`find_foods`, `get_food`, `lookup_barcode`, `upsert_food`,
`upsert_recipe`, `log_meal`, `update_meal`,
`delete_meal`, `log_metric`, `delete_metric`, `checkin`,
`update_profile_notes`, `propose_changes`, `apply_proposal`,
`reject_proposal`, `revert_proposal` and `undo_last`, the MCP server
that serves them at `/mcp`, with the coaching sessions as its prompts
and the methodology as its resource, the API contract and the web app's frame
(installed as an app, a PWA whose shell opens without a network and
whose new version a toast offers; signing in, in a card under the
product's mark and what it is, and out, an account made by an invite's link (a password
or the provider), its
sections in a bar at the bottom on a phone (none in a form, stepping
down under a sheet, the settings an avatar in a page's header away),
a rail on a tablet and a sidebar on a desktop, Log a meal over the
screen shown, sheets with a handle, dragged down to close, keyboard
shortcuts (a skip to the content, G and a letter to a section, ⌘Z or
Ctrl+Z to undo the latest change, W a weigh-in, and ⌘K or Ctrl+K the
command palette of the actions (yesterday's meal again among them), the
sections, foods, days and settings, each row with its key, which
typing on the day opens too; those of one key turned off by a switch
of the user's), a light and a dark theme,
each change said in a toast whose Undo takes it back, clear of a
sheet's buttons, a delete's for 10 s, and after it the recent changes
(a page of the settings, or a sheet from the undo's toast), by whom, with
Undo on the next, each number of
calibration's an ⓘ away from what it is, a page that loads a skeleton
of its shape under its header, one that did not load saying so there
with a way to ask again, a place with nothing yet an icon, a sentence
and the action that fills it where there is one, and what came kept,
under a bar that says since when, when it cannot be asked again, in
English and Croatian)
with the day (shown at once, with what it shows above its meals, so
nothing moves them after; under a header of one row, ‹ the title ›, a
Today pill on another day and the user's mark; a new user's first
steps to a plan in place of its hero,
ticked one by one under a bar of them, the AI coach's step after them
a card of what to write it, copied in a tap, and on a desktop how
tdeemon works beside them; what is left of the target in a ring, the one
number of what is left (the button that logs a meal repeats it), and
the macros' bars,
a day past it by more than 5 % in a warm colour and a word, not an error's,
with the target's arithmetic, the TDEE less the deficit (until there is
a TDEE, calibration's counts toward it), the day's type in a small
picker and the phase; under it, by the hour, the morning's weigh-in, a
proposal that waits in the coach's colour ("Claude proposes 2 plan
changes") or the evening's end of the week; a day before the plan
saying when it starts, the week's days, Monday to Sunday, as columns
of what each ate with the target's tick and today outlined, at every
width, and their average against the target, whose sheet
says what the days left can eat; one that did not load keeping its
header and its week; the morning summary
in a sheet (its notification's tap, or today's header), the meals as
rows of one card that open to their items, its last row the way to add
one, on a desktop a table with its total and that way at its foot
beside the
week, which the arrows move through, Enter opening a meal's editor, R
repeating it and Delete deleting it, a meal logged or repeated taking
the focus, the body and the check-in, the check-in a row that opens on a
phone (and until the user has measured anything), its scales five
steps with the step's word under them, with
calibration as a switch,
the body's tiles each a tap from its measurement, a missing one said in
words, and all the
measurements in a sheet, sleep in hours and minutes, a meal's
corrections in a sheet that
says its total as it changes, an item's amount with its unit and its
other ways in a menu, its day picked by name and its time on the
language's clock, a meal repeated or deleted taken there, a meal logged by hand
in a sheet titled by the meal of the day (a side panel of 440 px on a
desktop), of the foods found (the catalog's after them, marked), eaten
lately or most and the meals of before, those eaten at the meal of the
day lately first and today's last, the first of them as cards on top of
the first list, none said twice, whose search's keyboard waits
on a phone for a tap, a food's + putting it in the tray in the amount
its row says while the search stays, the tray there with its first item
(their count, kcal and macros, the amounts to edit, and "Log snack",
⌘↵ on a keyboard), a new food, an estimate (kcal alone, from its chip,
logged in three actions) or a barcode (the
camera's, or its digits typed) a step of its own with one button, or
a weigh-in from its chip, a recipe found marked, in a portion, a share
of it typed as a fraction (1/10) or chosen in its chips (½, ⅓, ¼, 1/…)
or grams, ≈ on the grams of one not weighed, opened there on its amount
from its page, and the chips under the meals that add
yesterday's meal or a food in a tap, their row fading at a phone's
edge; a share of a recipe said as a
fraction, ⅒ of the recipe, and its portions in the language's plural,
wherever an item's amount is),
the foods (their list by how often they are eaten, grouped by this
week and earlier, in use, the recipes, archived or the estimates,
searched within the list chosen, a row its kcal and protein for its
basis and no initials, marked only as a recipe or an estimate, a
recipe's a portion's, a new food and a new recipe each a button, and
before a recipe what one is for; a recipe's page with a portion's
energy and its split, 100 g's and
the whole dish's, whether it was weighed said plainly, ≈ on its grams
alone, its ingredients as given, one whose food changed since marked
with its new values a tap away, logged (Log 1 portion, in the
language's case), cooked again in a sheet of only
its amounts, its weight from the pot whose empty weight is remembered
and its portions, weighed in one, archived with Undo, a household's
card of its members, and edited (an icon beside them) in a
page of its own, its ingredients found in a sheet as a meal's are, each
in its unit, moved or removed, its weight, from the pot or "I can't
weigh it", warned of when far off its raw ingredients', its portions
and a note, what it comes to said at its foot as it is typed; a food's
page with its energy and its split, its
nutrition per 100 g or a serving, its barcodes, how often it was
logged, logged again from its last amount, changed first, archived
with Undo, and corrected in its own form, which warns when the kcal
and the macros disagree and adds a barcode, scanned or typed, or takes
one away;
beside the list on a desktop, each scrolled on its own, the page not,
its title staying at the top and its cards in two columns), the
progress (the TDEE measured as its
hero, ± its interval, whether it is reliable and the sum it comes of,
with the phase's verdict under it, or of a new phase a bar to its
first, or until there is a TDEE calibration on its way to one, its
counts against what it needs and about when a first comes; the weight,
its goal drawn only near it, and the TDEE in its band against the intake,
more layers (the space between them shaded and said, the target) a
switch, as charts from their first data, each a table of its days a
tap away, a proposal that waits marked on the chart it cites and a
line to the plan, the calendar weeks with data in a table, sleep, the
heart, the steps and the body fat in small, the last 7 days against
the usual and its range said, each tile a button to a chart, over the
phase by default (all the days while it is young), 30 or 90 days, a
year or all; ECharts loaded with them, in the theme's tokens), the plan
(its phase as the hero, with its pace,
week, the way to its goal as a bar (or its days) and the phases on a
timeline, edited and the next one
planned in a sheet, a proposal that waits, saying only what it changes
(now, new and the difference, in a table), who made it, when it
expires, why in three lines, the limits' warnings each rule once, in a
row that opens, and what the data say beside it, Accept and Reject at
its foot (A and X on a desktop), each answer said in a toast, marked
on the plan's section and in a banner on the day, the targets by type of
day (one, every day's, without training days, but a training target
of its own for a workout's day, and a way to add them)
and the deficit they plan, edited with the training days in a sheet,
the history of its revisions, a page of its own (its rows beside the
one open on a desktop, the latest beside the plan) with a revert said
in a toast, their rationales of the app's own as codes, a
proposal of the limits accepted there alone, the autopilot's card (its
mode, its last weekly review said from its numbers, the next review,
the drift against its limit or the step it may propose) and its steps
in the history by Autopilot, its proposals in the proposal's card,
the review's line on the day of it,
and, before a plan, how one comes; what it rests on of the user's, each
a row to its page: the profile, the limits in groups that open, a wrong
one focused at its field, the autopilot's settings (asking before it is
turned on, the review's weekday in seven
segments, what it may and never changes and when
it waits), the notes for a coach) and the settings as a
list under the account's card, beside the page open of it on a desktop,
signing out last and the version and the export at its foot (the
theme, the account with a time zone found by its name, its
data exported as a ZIP and the account deleted once its address is typed,
the admin's users (someone invited in a sheet whose link is shown once,
the invites waiting, each revoked, the accounts deactivated, made an admin
or deleted, each after it asks), the household (its members and the
way out, someone invited in a sheet, the invites sent, each revoked,
and those for the user's answer, declined or seen in a sheet of what
joining does and joined, a stale merge shown anew; its invite a line on
the day, its food's page saying a change is every member's, its
recipe's members marked), the
devices and AI coaches connected in a guided sheet, an access key (a
token) shown once with the steps to give it (an AI coach's MCP address
and commands, Health Auto Export's checklist) and its first call seen
as it comes, the notifications (this device's turned on at a tap, or
why it cannot, an iPhone's way to the app installed among them, the
devices notified, each removed apart, a test; the service worker shows
them and a tap opens their page), the recent changes, a page of their
own, signing out),
each screen checked for
access (axe, a finger's targets, the focus, less motion) on a phone and
a desktop, in
both themes and languages, exist.

| Path | What |
| --- | --- |
| `cmd/tdeemon` | the binary: the server (configuration, wiring, start and stop) and the `user create`, `user set-password`, `user set-telegram`, `user set-locale`, `user delete` and `user unlink` commands |
| `internal/config` | settings from `TDEEMON_*` environment variables |
| `internal/domain` | the domain types and their rules, one package each: `user`, `day` (the logical day, its status and its calendar week), `food` (foods, nutrients and barcodes), `meal` (items, totals, warnings), `recipe` (a dish of the user's foods that is a food of its own, its values per 100 g of the cooked dish, its servings the whole and a portion), `metric` (a day's measurements and their merge rules), `profile` (sex, birth date and height, and the age, BMI, BMR and protein's reference mass that follow), `plan` (phases and their rates, targets by type of day, the type of a day), `limits` (the three levels of limits of a plan, and `Validate`, whose contract is docs/limits.md), `proposal` (a proposed change of the plan, the user's limits among it, each change with the state before and after, its life (pending, applied, rejected, expired, superseded, reverted) and its check against the limits; DESIGN.md §8), `calibration` (the trend weight, the measured TDEE with its interval, of the window up to a day or of a target's days, what it needs and about when a first comes, and the verdict on a phase, DESIGN.md §7), `autopilot` (the weekly review: its settings, its history of waits and drift from the plan's revisions, and `Decide`, a step, a hold, a proposal or no step and why; DESIGN.md §8), `recovery` (the last week's sleep, resting heart rate and HRV against the weeks before, and their usual range), `event` (who wrote what, for the audit trail and undo), `clientid` (the ids a client makes for a meal or an undo, so a retry is known), `text` (the rules for free text, shared by the others) |
| `internal/auth` | credentials: password rules, argon2id hashes, session and API tokens, scopes, sealed values and who a request acts for |
| `internal/ingest` | what devices and apps send (Health Auto Export, a simple JSON form) turned into a day's metrics: which value a day keeps, units, a check of each value; the days of workouts in Strong's CSV export |
| `internal/coaching` | the coaching sessions (a monthly review, a plateau check, a new phase): the steps of each in `sessions/`, given with the methodology |
| `internal/telegram` | the instance's Telegram bot: sends a message, reads the bot's messages (long polling), sets its commands |
| `internal/off` | looks a product up by its barcode at Open Food Facts and makes a new food of it |
| `internal/notify` | sends a user a notification to each of their channels: their Telegram chat, following a group to its supergroup, and by Web Push each of their devices, a gone one deleted; it fails only when none took it |
| `internal/push` | Web Push: a message encrypted for a browser's subscription (RFC 8291) and signed by the instance's VAPID key, derived from the secret key, to the push services alone |
| `internal/watchdog` | the alarms when a user's data stop coming: no metric for 26 h, an ingest token unused for 3 days; to each channel of the user's |
| `internal/scheduler` | the jobs of each user's day (the autopilot's weekly review, the morning summary, the watchdog), each occurrence once in the usual case and at least once in any, recorded in the store; and the planner's statistics refreshed daily |
| `internal/store` | SQLite access, the instance's catalog of declared foods among it, and the migrations in `migrations/` |
| `internal/store/storetest` | a fresh, migrated database file for tests |
| `internal/api` | the HTTP API (huma), whose spec is `api/openapi.json`: sign-in, sessions, API tokens, the profile and the limits of a plan, the user's own change of it, the settings, the devices' push subscriptions and a test push, the list of foods, a new food, a new recipe, a food's use, the recent meals and the recent changes (web session only), the imports (API token only), and the tools, a barcode's food (the user's, the catalog's, or Open Food Facts' product) among them, the history, the plan, its proposals, the notes and a coaching session's guide among them (`tools.go`, the registry; `mcp.go` serves it at `/mcp`) |
| `internal/server` | the HTTP server and graceful shutdown |
| `internal/webui` | serves the built web app (build tag `webui`) |
| `web/` | the React app (Vite, TypeScript 7, TanStack, Tailwind); its text in each language is in `web/src/i18n`, its end-to-end tests in `web/e2e` |
| `web/codegen` | generates `web/src/api/schema.d.ts` from the spec |
| `docs/` | the design, the conventions, the limits of a plan with their sources (`limits.md`), and how the numbers come about for a coach (`methodology.md`, which the binary serves as it is, through `docs.go`) |
| `.github/` | the CI workflows, the ruleset for `main`, issue and PR templates |
| `scripts/` | `next-version.sh` for releases and `image-tags.sh` for their image, with their tests; `e2e-server.sh`, the server of the end-to-end tests; `apply-github-settings.sh`, for the maintainer only |

## Commands

```sh
mise install       # every tool at the pinned version (mise.toml, mise.lock)
lefthook install   # git hooks, once per clone
make check         # what CI checks, except a few that only CI runs
make e2e           # the end-to-end tests in Chromium (make e2e-browser once)
make fmt           # Go and the web app; a new migration: sqlfluff fix <file>
make help          # the other targets
```

Without mise activated in the shell, run the tools as
`mise exec -- <tool>`. Run the server with `TDEEMON_BASE_URL`,
`TDEEMON_SECRET_KEY` and `go run ./cmd/tdeemon`; CONTRIBUTING.md has the
details.

## Rules

- **Never commit to `main`.** Work on a branch and open a pull request.
  It is squash-merged, so its title, a Conventional Commit
  (`feat: …`, `fix(store): …`, `!` for breaking changes), becomes the
  commit on `main`.
- **No AI attribution** in commits or pull requests: no `Co-Authored-By`
  trailers for AI tools and no "Generated with" lines.
- **No personal data in the repo, ever.** The repo is public. Real
  weights, meals, health exports and chat sessions stay out of git; test
  fixtures are made up or anonymised. No secrets either.
- **A vulnerability stays out of public view.** One you find stays out
  of anything pushed to GitHub (issues, pull requests, branches and
  commits) until the maintainer lands the fix, as docs/conventions.md
  describes; tell the maintainer in the session.
- **Never run `scripts/apply-github-settings.sh`**, except with
  `--dry-run`. It changes the live repository's settings and the ruleset
  for `main` with admin rights; the maintainer runs it.
- **Tests come with every change in behaviour**, and they fail without
  it. Use a real SQLite file (`storetest.Open`), not mocks of the
  standard library. Coverage floors are in `.testcoverage.yml` and
  `web/vite.config.ts`.
- **Generated files stay fresh:**
  - after changing the API, regenerate `api/openapi.json`
    (`go test ./internal/api -update`) and the TypeScript types
    (`(cd web/codegen && npm ci && npm run gen:api)`);
  - after a migration, refresh the schema snapshot
    (`go test ./internal/store -run TestSchema -update`);
  - after `food.Key` or `golang.org/x/text` changes, the web app's table
    of names' keys
    (`go test ./internal/domain/food -run TestWebKey -update`);
  - after a limit the web app checks a form by changes (a metric's
    bounds, a meal's or a food's limits, the profile's, a name's
    length), its copy `web/src/lib/limits.json`
    (`go test ./internal/api -run TestWebLimits -update`).
- **API tokens are kept out by default.** An operation refuses API
  tokens unless its `Metadata` opens it to a scope
  (`openToTokens(auth.ScopeIngest)`, in `internal/api/tokens.go`);
  settings (the notes for a coach aside, a tool's) and anything about
  tokens or sessions stay closed to them.
- **One user's data stays theirs.** A store method for a user's rows
  takes the user's ID and filters by it; a handler takes the user from
  the request's principal. The one exception is a recipe its users
  shared, and every food and recipe of a household's members, which a
  member's save writes into the others' copies, as docs/conventions.md
  says. A new operation in the OpenAPI document adds
  its case to `isolationCases` in `internal/api/isolation_test.go`, or
  the test fails, and a new kind of data adds rows of it for both users
  to `newIsolationEnv`; a new tool adds its case to
  `mcpIsolationCases` in `internal/api/mcp_isolation_test.go`, which MCP
  holds to (`docs/conventions.md`).
- **Answers match the document.** Tests check every answer against
  `api/openapi.json`; a new operation adds its case to `answerCases` in
  `internal/api/schema_test.go`, or the test fails. In an answer's type
  a pointer has `omitempty` and a slice `nullable:"false"`, and a
  handler returns `internalError(msg, err)` instead of logging it
  (`docs/conventions.md`).
- **Migrations only go forward.** Never edit a migration once it is
  merged; add a new one. The rules are in
  `internal/store/migrations/README.md`.
- **Follow the linters, do not silence them.** A disable is for a linter
  that is wrong in that place, and it says which rule and why:
  - Go: `//nolint:<linter> // why` (nolintlint enforces both);
  - web: `// oxlint-disable-next-line <rule> -- why`, and the same for
    `eslint-disable` (oxlint and `scripts/check-disable-comments.sh`
    enforce both).
- **Code, comments, docs and commit messages are in English.**
- **Keep the docs true:** update the README, docs/conventions.md,
  docs/DESIGN.md or docs/limits.md when a change makes them wrong; a
  limit in the code and its row in docs/limits.md change together.

## Definition of Done

A change is done when its behaviour is tested, `make check` and CI
pass, generated files and docs are up to date, nothing personal entered
the repo, and the pull request says what changed, why and how it was
checked.
