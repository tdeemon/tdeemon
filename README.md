# tdeemon

Self-hosted nutrition tracking with an adaptive TDEE, made to work with
an AI coach.

Log meals from the web app or from any MCP client (Claude Code, Codex,
Hermes, a local model). tdeemon does the math: it learns your
real energy expenditure from what you eat and how your weight moves,
and lets an AI coach adjust your plan (cut, maintenance, bulk or recomp)
only through proposals you accept, within limits enforced in code.

![The day on a desktop: what is left of the target, the macros, the week,
an AI coach's proposal and the meals](docs/screenshots/desktop-day.png)

![Progress: the measured TDEE, the verdict on the phase, the TDEE
against the intake and the weight's trend](docs/screenshots/desktop-progress.png)

The screenshots show `tdeemon user demo`, three months of made-up data.

> **Status: early.** tdeemon runs every day for its author, and this is
> its first release for other self-hosters. Expect changes between
> releases, each said in its notes.

## What it does

- **A day at a glance:** what is left of the target, the macros, the
  week, the meals; logging by search, a barcode (the camera, or Open
  Food Facts), a recipe's portion, a quick estimate, or yesterday's meal
  again.
- **An adaptive TDEE:** measured from intake and the weight's trend, with
  its interval and a verdict on the phase, explained number by number
  ([methodology](docs/methodology.md)).
- **A plan with limits:** phases, targets by type of day, and limits
  nobody can lower below a floor in the code ([limits](docs/limits.md));
  an AI coach proposes changes, you accept or reject them, and anything
  is undone.
- **Autopilot:** a weekly review that steps the plan within your limits,
  or asks.
- **An AI coach over MCP:** the tools, the coaching sessions and the
  methodology, for any MCP client and model.
- **Devices:** Apple Health through Health Auto Export, workouts from
  Strong, a morning summary and alarms on Telegram or as push
  notifications.
- **For a household:** several users on one instance, a shared kitchen
  of foods and recipes, each person's meals, weight and plan their own.
- **One container, one SQLite file**, an installable web app (PWA), in
  English and Croatian.

## Quick start

With Docker and Compose, on the machine that will run it:

```sh
mkdir tdeemon && cd tdeemon
base=https://raw.githubusercontent.com/zarnautovic/tdeemon/main/deploy/compose
curl -fsSLO "$base/compose.yaml" -O "$base/compose.caddy.yaml" -O "$base/Caddyfile" -O "$base/.env.example"

# The settings, readable by you only, with a new secret key. Keep the key.
(umask 077 && set -C && sed "s|^TDEEMON_SECRET_KEY=\$|TDEEMON_SECRET_KEY=$(openssl rand -base64 32)|" \
  .env.example > .env)
# In .env, set TDEEMON_BASE_URL to the address it will have, such as
# https://tdeemon.example.com, or http://localhost:8080 to try it here.

docker compose up -d --wait
docker compose exec tdeemon /tdeemon user create --admin --email you@example.com
```

The last command prints your password once. Open the base URL and sign
in; the first steps on the day take you to a plan. To look around with
three months of made-up data first, add a demo user:
`docker compose exec tdeemon /tdeemon user demo --email demo@example.com`.

tdeemon then listens on `127.0.0.1:8080`. For HTTPS, put your reverse
proxy in front of it, or let Caddy do it: uncomment `COMPOSE_FILE` in
`.env` and run `docker compose up -d --wait` again
([Docker Compose](docs/self-hosting.md#docker-compose)).

## Documentation

- [Self-hosting](docs/self-hosting.md): configuration, users, devices,
  an AI coach, backups and upgrades.
- [Methodology](docs/methodology.md): how the numbers come about.
- [Limits](docs/limits.md): the limits of a plan and their sources.
- [Design](docs/DESIGN.md): the whole design.
- [Contributing](CONTRIBUTING.md), and [security](SECURITY.md): report a
  vulnerability privately.

## Disclaimer

tdeemon is not medical advice. Talk to a professional before making
significant changes to your diet or training.

## License

[AGPL-3.0-only](LICENSE). Contributions come under the same licence, with no
CLA to sign ([CONTRIBUTING.md](CONTRIBUTING.md)).
