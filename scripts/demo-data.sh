#!/usr/bin/env bash
#
# Loads ten demo profiles (Genoa / Milan) with generated photos so Discover has
# people to show. Local/dev only: it signs in through the developer login, which
# production deliberately does not have.
#
#   make dev-up && make run     # API on :8080 with DEV_AUTH=true
#   make demo                   # this script
#
# Safe to re-run: profiles that already exist are skipped. Afterwards sign in
# with any Telegram id 700001..700010 on the developer sign-in screen.
set -euo pipefail
cd "$(dirname "$0")/.."

command -v node >/dev/null 2>&1 || { echo "node (v18+) is required." >&2; exit 1; }

BASE="${BASE:-${API_URL:-http://127.0.0.1:8080}}"
code=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 "$BASE/healthz" 2>/dev/null) || code=000
if [[ "$code" != "200" ]]; then
  echo "The API is not reachable at $BASE. Start it with: make dev-up && make run" >&2
  exit 1
fi

dev=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 5 -X POST -H 'Content-Type: application/json' -d '{}' "$BASE/api/auth/dev" 2>/dev/null) || dev=000
if [[ "$dev" == "404" || "$dev" == "405" ]]; then
  echo "DEV_AUTH is off on $BASE — demo data can only be loaded into a development API." >&2
  exit 1
fi

BASE="$BASE" node backend/scripts/seed.mjs
