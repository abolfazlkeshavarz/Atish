#!/usr/bin/env bash
# Health check for every Atish service.
#
#   bash scripts/health.sh                          # the local docker stack
#   API_URL=https://x REMOTE=1 bash scripts/health.sh   # only the public API of a deployment
#
# Exits non-zero if anything required is down, so it works as a post-deploy gate.
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
# shellcheck disable=SC1091
source scripts/lib.sh
[[ -f .env ]] && load_env .env 2>/dev/null

API_URL="${API_URL:-http://127.0.0.1:${WEB_PORT:-8080}}"
REMOTE="${REMOTE:-}"
DOCKER="${DOCKER:-docker}"
COMPOSE=("$DOCKER" compose)

GREEN=$'\033[0;32m'; RED=$'\033[0;31m'; YELLOW=$'\033[1;33m'; NC=$'\033[0m'
failures=0; warnings=0
ok()   { echo "  ${GREEN}[OK]${NC}   $1"; }
bad()  { echo "  ${RED}[FAIL]${NC} $1"; failures=$((failures + 1)); }
warn() { echo "  ${YELLOW}[WARN]${NC} $1"; warnings=$((warnings + 1)); }

http_code() { local c; c=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 10 "$1" 2>/dev/null) || c=000; echo "${c:-000}"; }

check_api() {
  echo ""
  echo "Atish API ($API_URL)"
  local code
  code=$(http_code "$API_URL/healthz")
  [[ "$code" == "200" ]] && ok "GET /healthz -> 200" || { bad "GET /healthz -> $code"; return; }

  code=$(http_code "$API_URL/api/me")
  [[ "$code" == "401" ]] && ok "protected route rejects anonymous requests" || bad "GET /api/me without a token -> $code (expected 401)"

  code=$(http_code "$API_URL/api/catalog")
  [[ "$code" == "200" ]] && ok "public catalog served" || bad "GET /api/catalog -> $code"

  code=$(http_code "$API_URL/")
  [[ "$code" == "200" ]] && ok "web app served" || warn "GET / -> $code (is the web container up?)"

  code=$(curl -sS -o /dev/null -w "%{http_code}" --max-time 10 -X POST -H "Content-Type: application/json" -d "{}" "$API_URL/api/auth/dev" 2>/dev/null) || code=000
  if [[ "$code" == "404" || "$code" == "405" ]]; then
    ok "developer login is disabled"
  else
    bad "/api/auth/dev answered $code — DEV_AUTH must be off in production"
  fi
}

echo ""
echo "Atish health check"
echo "=================="

if [[ -n "$REMOTE" ]]; then
  check_api
else
  echo ""
  echo "Containers"
  if ! command -v "$DOCKER" >/dev/null 2>&1; then
    warn "docker not found; skipping container checks"
  else
    for svc in postgres redis backend web; do
      cid="$("${COMPOSE[@]}" ps -q "$svc" 2>/dev/null | head -1)"
      if [[ -z "$cid" ]]; then bad "$svc is not running (try: make up)"; continue; fi
      state="$("$DOCKER" inspect -f '{{.State.Status}}' "$cid" 2>/dev/null)"
      health="$("$DOCKER" inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$cid" 2>/dev/null)"
      if [[ "$state" == "running" && ( "$health" == "healthy" || "$health" == "none" ) ]]; then
        ok "$svc ($state${health:+, $health})"
      elif [[ "$state" == "running" ]]; then
        warn "$svc running but health=$health"
      else
        bad "$svc is $state"
      fi
    done

    echo ""
    echo "PostgreSQL"
    pgc="$("${COMPOSE[@]}" ps -q postgres 2>/dev/null | head -1)"
    if [[ -n "$pgc" ]]; then
      q() { "$DOCKER" exec -i "$pgc" psql -U atish -d atish -tAc "$1" 2>/dev/null; }
      if [[ "$(q 'SELECT 1')" == "1" ]]; then
        ok "accepting connections"
        for t in users profiles matches messages plans; do
          if [[ "$(q "SELECT to_regclass('public.$t') IS NOT NULL")" == "t" ]]; then
            ok "table $t ($(q "SELECT count(*) FROM $t") rows)"
          else
            bad "table $t is missing (the API applies migrations at boot — check: make logs SERVICE=backend)"
          fi
        done
      else
        bad "cannot query the database"
      fi
    fi

    echo ""
    echo "Redis"
    rc="$("${COMPOSE[@]}" ps -q redis 2>/dev/null | head -1)"
    if [[ -n "$rc" ]]; then
      [[ "$("$DOCKER" exec "$rc" redis-cli ping 2>/dev/null | tr -d '\r')" == "PONG" ]] && ok "PING -> PONG" || bad "PING failed"
    fi
  fi
  check_api
fi

echo ""
echo "=================="
if (( failures > 0 )); then
  echo "${RED}${failures} failed${NC}, ${warnings} warning(s)"; exit 1
fi
echo "${GREEN}All required checks passed${NC}, ${warnings} warning(s)"
