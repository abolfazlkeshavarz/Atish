#!/usr/bin/env bash
# Pre-flight checks to run BEFORE deploying Atish.
#
#   make deploy-check
#
# Catches the mistakes that are cheap to find now and expensive in production:
# committed secrets, weak or default values, code that does not compile.
set -uo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
# shellcheck disable=SC1091
source scripts/lib.sh

GREEN=$'\033[0;32m'; RED=$'\033[0;31m'; YELLOW=$'\033[1;33m'; NC=$'\033[0m'
errors=0; warnings=0
ok()   { echo "  ${GREEN}[OK]${NC}    $1"; }
bad()  { echo "  ${RED}[BLOCK]${NC} $1"; errors=$((errors + 1)); }
warn() { echo "  ${YELLOW}[WARN]${NC}  $1"; warnings=$((warnings + 1)); }

echo ""
echo "Atish deploy pre-flight"
echo "======================="

echo ""
echo "1. Secrets"
if git ls-files --error-unmatch .env >/dev/null 2>&1; then
  bad ".env is tracked by git — it holds credentials (fix: git rm --cached .env)"
else
  ok ".env is not tracked"
fi

if [[ ! -f .env ]]; then
  warn ".env not found — create it with: make env"
else
  load_env .env 2>/dev/null
  [[ "${APP_ENV:-}" == "production" ]] && ok "APP_ENV=production" || warn "APP_ENV is '${APP_ENV:-unset}', expected production"
  [[ "${DEV_AUTH:-false}" == "true" ]] && bad "DEV_AUTH=true — anyone could sign in as any user" || ok "DEV_AUTH is off"
  [[ ${#JWT_SECRET} -ge 32 ]] && ok "JWT_SECRET is ${#JWT_SECRET} chars" || bad "JWT_SECRET must be at least 32 characters"
  if [[ -n "${ENCRYPTION_KEY:-}" ]] && [[ "$(printf '%s' "$ENCRYPTION_KEY" | base64 -d 2>/dev/null | wc -c)" == "32" ]]; then
    ok "ENCRYPTION_KEY decodes to 32 bytes"
  else
    bad "ENCRYPTION_KEY must be base64 of 32 bytes (openssl rand -base64 32)"
  fi
  case "${POSTGRES_PASSWORD:-}" in ""|change-me|atish|postgres|password) bad "POSTGRES_PASSWORD is empty or a default" ;; *) ok "POSTGRES_PASSWORD is set" ;; esac
  if [[ -z "${ADMIN_PASSWORD:-}" ]]; then
    warn "ADMIN_PASSWORD empty: password login for /admin is disabled (use ADMIN_TELEGRAM_IDS)"
  elif [[ ${#ADMIN_PASSWORD} -lt 12 ]]; then
    bad "ADMIN_PASSWORD must be at least 12 characters"
  else
    ok "ADMIN_PASSWORD is strong enough"
  fi
  [[ -n "${TELEGRAM_BOT_TOKEN:-}" ]] && ok "TELEGRAM_BOT_TOKEN set" || warn "TELEGRAM_BOT_TOKEN is empty — Telegram login will not work"
  [[ "${PUBLIC_URL:-}" == https://* ]] && ok "PUBLIC_URL is https" || warn "PUBLIC_URL is '${PUBLIC_URL:-unset}' — Telegram Mini Apps require https"
fi

echo ""
echo "2. Build"
if (cd backend && go build ./... >/dev/null 2>&1); then ok "backend compiles"; else bad "backend does NOT compile (cd backend && go build ./...)"; fi
if (cd backend && go vet ./... >/dev/null 2>&1); then ok "go vet is clean"; else warn "go vet reported issues"; fi
if (cd backend && go test ./... >/dev/null 2>&1); then ok "backend tests pass"; else bad "backend tests fail (cd backend && go test ./...)"; fi
if [[ -d frontend/node_modules ]]; then
  if (cd frontend && npx tsc --noEmit >/dev/null 2>&1); then ok "frontend typechecks"; else bad "frontend does NOT typecheck"; fi
else
  warn "frontend/node_modules missing — run: cd frontend && npm install"
fi

echo ""
echo "3. Repository hygiene"
tracked_bad="$(git ls-files 2>/dev/null | grep -E '(^|/)(\.env|node_modules/|dist/|data/)|\.exe$|\.log$' | grep -v '\.env\.example' | head -5)"
if [[ -n "$tracked_bad" ]]; then
  bad "files that should never be committed are tracked:"; sed 's/^/           /' <<<"$tracked_bad"
else
  ok "no env files, build output, binaries or logs are tracked"
fi
uncommitted="$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')"
[[ "$uncommitted" != "0" ]] && warn "$uncommitted uncommitted change(s) — the server builds what you push"

echo ""
echo "======================="
if (( errors > 0 )); then
  echo "${RED}${errors} blocker(s)${NC}, ${warnings} warning(s) — do not deploy yet"
  exit 1
fi
echo "${GREEN}No blockers${NC}, ${warnings} warning(s)"
echo ""
echo "Next, on the server:  DOMAIN=... LETSENCRYPT_EMAIL=... ./scripts/bootstrap-vps.sh"
echo "Low-resource VPS:     make images-bundle  ->  scp  ->  make load-images  ->  ./scripts/bootstrap-vps.sh"
