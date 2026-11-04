# AI coach

tdeemon is an MCP server: any MCP client that takes tools, with any
model, logs your meals, reads your days and coaches you through it.
The model decides only what you ate and how much. tdeemon finds the
foods, logs them, adds them up and checks every change against your
limits, so the model never does the arithmetic and can't push your plan
past them. A change of the plan is a proposal, applied only after your
yes.

## An example

A made-up session in Claude Code; the tools it calls are in brackets.

```text
You:    Lunch: 150 g chicken breast, 200 g rice and a tablespoon of olive oil
Claude: [find_foods "chick", "rice", "oliv"] [log_meal]
        Logged lunch at 13:05:
        - Chicken breast, roasted 150 g: 248 kcal, P 47 · C 0 · F 5
        - White rice, cooked 200 g: 260 kcal, P 5 · C 56 · F 1
        - Olive oil 1 tbsp (10 g): 88 kcal, F 10
        Today: 1,342 of 2,100 kcal eaten, 758 left; protein 112 of 195 g.

You:    How am I doing this week?
Claude: [get_summary]
        Average 2,060 kcal against 2,100, protein on target on 5 of 6
        days. Your trend weight fell 0.4 kg; the measured TDEE is 2,602
        ± 83 kcal, so the cut runs at -0.3 % a week, under its range of
        0.4 to 0.6 %.

You:    Let's do the monthly review.
Claude: [get_coaching_guide monthly_review] [get_history] [get_plan]
        ... the review ...
        I propose 100 kcal less on every day, carbs only.
        [propose_changes]  It waits for you on the plan.
```

The proposal then waits in the app, with its reasons and the limits it
comes close to, until you accept or reject it.

## Connect a client

Any MCP client that takes tools reaches tdeemon, with any model: a
client from Anthropic or OpenAI, an open model such as DeepSeek, or a
local one. The address is `<TDEEMON_BASE_URL>/mcp` (MCP over Streamable
HTTP), with an API token as a Bearer token: one with `tools` for every
tool, or `read` for those that only read. Each tool is an operation of
the REST API, with its validation and its answers, and a meal or a
change made through MCP is the client `mcp`'s. The server tells every
client how to log (its MCP instructions), so a client's own skill, if
any, shrinks to a few lines.

The app's **Settings › Devices and AI coaches** gives the token, the
address, which is the instance's (`TDEEMON_BASE_URL`, which
`GET /api/v1/info` says as `base_url`), and the commands for Claude
Code and Claude Desktop. For a few clients:

- **Claude Code:**

  ```sh
  claude mcp add --transport http tdeemon https://tdeemon.example.com/mcp \
    --header "Authorization: Bearer tdm_…"
  ```

- **Claude Desktop:** its configuration, through
  [mcp-remote](https://www.npmjs.com/package/mcp-remote), which passes
  the header on (the app's Connect gives it).
- **Codex:** the token in an environment variable of the shell Codex
  runs in, then

  ```sh
  export TDEEMON_TOKEN=tdm_…
  codex mcp add tdeemon --url https://tdeemon.example.com/mcp \
    --bearer-token-env-var TDEEMON_TOKEN
  ```

- **Hermes Agent:** in its `config.yaml`, the token in its environment
  (`.env`):

  ```yaml
  mcp_servers:
    tdeemon:
      url: https://tdeemon.example.com/mcp
      headers:
        Authorization: Bearer ${MCP_TDEEMON_API_KEY}
  ```

  Hermes finds a server's tools by searching them and does not pass the
  server's instructions to its model: its own skill names the tools it
  should use, `get_coaching_guide` for a coaching session among them.

To check a token and the address before a client, ask the server what
it is (MCP's `initialize`; the server keeps no session between
requests):

```sh
curl -H 'Authorization: Bearer tdm_…' -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}' \
  https://tdeemon.example.com/mcp
```

It answers with `serverInfo` (`tdeemon` and its version) and the
instructions; a bad token, 401.

ChatGPT's and claude.ai's connectors sign in with OAuth rather than a
header, which tdeemon does not offer; whether they would work, on a
phone above all, is not checked yet. The built-in chat (planned) is the
way in from a phone.

## The tools

| Tool | What it does |
| --- | --- |
| `get_profile` | the profile, the plan's settings and the time now |
| `get_day` | a day: its meals, totals, target, what is left, measurements |
| `get_summary` | the weeks up to yesterday: averages, trend, TDEE, verdict |
| `get_history` | days in a range: intake, weight, trend, calibration |
| `get_plan` | the phases, targets, limits, proposals and the autopilot |
| `get_coaching_guide` | a coaching session's steps, with the methodology |
| `find_foods` | the user's foods and recipes, then the catalog's |
| `get_food` | a food whole |
| `lookup_barcode` | a barcode's food: the user's, the catalog's or Open Food Facts' |
| `upsert_food`, `upsert_recipe` | a new food or recipe, or one changed |
| `log_meal`, `update_meal`, `delete_meal` | a meal logged, corrected or deleted |
| `log_metric`, `delete_metric` | a measurement, such as a weigh-in |
| `checkin` | the day's check-in |
| `update_profile_notes` | the notes for a coach |
| `propose_changes` | a proposed change of the plan |
| `apply_proposal`, `reject_proposal` | a proposal's answer |
| `revert_proposal` | a proposal to take a revision of the plan back |
| `undo_last` | the latest change undone |

A token of the `read` scope gets the tools that only read.

## Coaching sessions

Ask in words, in any client: "do a monthly review", "my weight has
stalled", "plan a bulk from January". The model calls
`get_coaching_guide` first, which answers the session's steps and
[how tdeemon reads the numbers](methodology.md), and follows them:
a monthly review, a plateau check or a new phase. A change of the plan
is a proposal, applied only after your yes. A client that shows MCP
prompts lists the same sessions as commands (in Claude Code,
`/mcp__tdeemon__monthly_review`, `…plateau_check`, `…new_phase`, with
what you want from it after them), and serves the methodology as the
resource `tdeemon://methodology`.
