#!/usr/bin/env bash
# Runs the server the docs' screenshots are taken of
# (web/playwright.screenshots.config.ts): the binary built with the web
# app, on 127.0.0.1:<port>, over a fresh data directory with one demo
# user of made-up data (tdeemon user demo), whose address and password
# are SHOTS_EMAIL and SHOTS_PASSWORD. Nothing in it is anyone's.
#
#   SHOTS_EMAIL=… SHOTS_PASSWORD=… scripts/screenshots-server.sh <binary> <port> <data dir>
set -euo pipefail

if [[ $# -ne 3 || ! $2 =~ ^[0-9]+$ || -z ${SHOTS_EMAIL:-} || -z ${SHOTS_PASSWORD:-} ]]; then
  echo "usage: SHOTS_EMAIL=… SHOTS_PASSWORD=… $0 <binary> <port> <data dir>" >&2
  exit 2
fi
bin=$1 port=$2 dir=$3

rm -rf "$dir"
mkdir -p "$dir"
export TDEEMON_DATA_DIR=$dir
export TDEEMON_BASE_URL=http://localhost:8080
export TDEEMON_LOG_FORMAT=text
export TDEEMON_BACKUP_KEEP=0
export TDEEMON_LOG_LEVEL=warn
export TDEEMON_OPEN_FOOD_FACTS_URL=off
TDEEMON_SECRET_KEY=$(head -c 32 /dev/urandom | base64)
export TDEEMON_SECRET_KEY

# The demo user needs the database, which the server creates when it
# starts: it starts first on another port, as in scripts/e2e-server.sh.
TDEEMON_LISTEN=127.0.0.1:$((port + 1)) "$bin" &
first=$!
up=false
for _ in $(seq 100); do
  if ! kill -0 "$first" 2>/dev/null; then
    break
  fi
  if curl -fs -o /dev/null "http://127.0.0.1:$((port + 1))/healthz"; then
    up=true
    break
  fi
  sleep 0.1
done
kill "$first" 2>/dev/null || true
wait "$first" || true
if [[ $up != true ]]; then
  echo "$0: the server did not start on port $((port + 1))" >&2
  exit 1
fi

printf '%s\n' "$SHOTS_PASSWORD" |
  "$bin" user demo --email "$SHOTS_EMAIL" --name Alex --timezone Europe/Zagreb --password-stdin >/dev/null

TDEEMON_LISTEN=127.0.0.1:$port exec "$bin"
