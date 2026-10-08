#!/usr/bin/env bash
# End-to-end smoke test against a running Atish API (onboarding, discovery
# rules, matching, chat, safety, premium gating, admin actions, deletion).
#
#   make dev-up && make run          # terminal 1 (API with DEV_AUTH=true)
#   make smoke                       # terminal 2
#
# The test signs in through the developer login and drives the admin API, so
# it needs DEV_AUTH=true and ADMIN_PASSWORD on the target — never run it
# against production (DEV_AUTH is refused there on purpose).
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

command -v node >/dev/null 2>&1 || { echo "node (v18+) is required to run the smoke test." >&2; exit 1; }

BASE="${BASE:-${API_URL:-http://127.0.0.1:8080}}"
code=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 "$BASE/healthz" 2>/dev/null) || code=000
if [[ "$code" != "200" ]]; then
  echo "The API is not reachable at $BASE (HTTP ${code:-none})." >&2
  echo "Start it with 'make run' (dev) or 'make up' (docker), or set BASE=..." >&2
  exit 1
fi

dev=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 -X POST -H 'Content-Type: application/json' -d '{}' "$BASE/api/auth/dev" 2>/dev/null) || dev=000
if [[ "$dev" == "404" || "$dev" == "405" ]]; then
  echo "DEV_AUTH is off on $BASE, so the smoke test cannot sign in." >&2
  echo "Use the dev stack:  make dev-up && make run" >&2
  exit 1
fi

export BASE
export ADMIN_PASSWORD="${ADMIN_PASSWORD:-supersecret-admin-pw}"
exec node backend/scripts/smoke.mjs
