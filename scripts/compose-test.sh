#!/usr/bin/env bash
# Starts deploy/compose with an image and checks it comes up healthy:
# tdeemon alone on 127.0.0.1:8080, then behind Caddy (plain HTTP, as the
# base URL is http://localhost, so Caddy asks no certificate), whose port
# 80 the test publishes on 127.0.0.1:8081 instead. CI runs it on the image
# a pull request builds; it needs Docker Compose and ports 8080 and 8081
# free.
#
#   scripts/compose-test.sh <image>
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <image>" >&2
  exit 2
fi
root=$(cd "$(dirname "$0")/.." && pwd)
dir=$(mktemp -d)
project=tdeemon-compose-test
cp "$root/deploy/compose/compose.yaml" "$root/deploy/compose/compose.caddy.yaml" \
  "$root/deploy/compose/Caddyfile" "$dir/"
# The recipe of README.md, "Docker Compose", with the base URL of this
# machine.
(cd "$dir" && umask 077 && set -C &&
  sed -e "s|^TDEEMON_SECRET_KEY=\$|TDEEMON_SECRET_KEY=$(openssl rand -base64 32)|" \
    -e 's|^TDEEMON_BASE_URL=.*|TDEEMON_BASE_URL=http://localhost|' \
    "$root/deploy/compose/.env.example" > .env)
# Caddy on a port of the test's: 80 and 443 may be taken on this machine.
cat >"$dir/compose.test.yaml" <<'EOF2'
services:
  caddy:
    ports: !override
      - 127.0.0.1:8081:80
EOF2

export TDEEMON_IMAGE=$1
# Run from the directory, as a self-hoster does: Compose reads .env, and
# COMPOSE_FILE in it, from there.
compose() { (cd "$dir" && docker compose -p "$project" "$@"); }
all=(-f compose.yaml -f compose.caddy.yaml -f compose.test.yaml)
cleanup() {
  status=$?
  if ((status != 0)); then
    compose "${all[@]}" logs || true
  fi
  compose "${all[@]}" down -v --remove-orphans || true
  # The network of tdeemon alone, which no service behind Caddy uses.
  compose -f compose.yaml down -v || true
  rm -rf "$dir"
  exit "$status"
}
trap cleanup EXIT

echo "== tdeemon alone"
compose up -d --wait --wait-timeout 180
curl -fsS --retry 5 --retry-connrefused http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/api/v1/info
echo

echo "== behind Caddy"
# As .env.example says, with the test's file too.
echo "COMPOSE_FILE=compose.yaml:compose.caddy.yaml:compose.test.yaml" >>"$dir/.env"
compose up -d --wait --wait-timeout 180
curl -fsS --retry 10 --retry-connrefused -H "Host: localhost" http://127.0.0.1:8081/healthz
# tdeemon publishes no port of its own any more.
if curl -fsS --max-time 5 http://127.0.0.1:8080/healthz >/dev/null 2>&1; then
  echo "tdeemon still answers on 127.0.0.1:8080 behind Caddy" >&2
  exit 1
fi

echo "== an upgrade behind Caddy keeps tdeemon on Caddy's network"
compose up -d --wait --wait-timeout 180 --force-recreate tdeemon
curl -fsS --retry 10 --retry-connrefused -H "Host: localhost" http://127.0.0.1:8081/healthz
echo "ok"
