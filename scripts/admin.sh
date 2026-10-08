#!/usr/bin/env bash
#
# Runs the Atish admin CLI against the right environment:
#   * the running production stack (docker compose backend container), or
#   * the local development database (make dev-up) via `go run`.
#
# Auto-detects by default; force with PROD=1 (container) or PROD=0 (dev DB).
#
#   scripts/admin.sh list-users -q alex
#   PROD=1 scripts/admin.sh set-status -user alex -status banned -reason spam
#
# Normally invoked through the Makefile: make admin-users Q=alex
set -euo pipefail
cd "$(dirname "$0")/.."

DOCKER="${DOCKER:-docker}"

use_prod="${PROD:-}"
if [[ -z "$use_prod" ]]; then
  if "$DOCKER" compose ps -q backend 2>/dev/null | grep -q .; then use_prod=1; else use_prod=0; fi
fi

if [[ "$use_prod" == "1" ]]; then
  # -T: no TTY, so output can be piped and the script works from CI/ssh.
  exec "$DOCKER" compose exec -T backend atish-cli "$@"
fi

command -v go >/dev/null 2>&1 || { echo "go is required for the dev CLI (or run the stack and use PROD=1)." >&2; exit 1; }

export DATABASE_URL="${DATABASE_URL:-postgres://atish:atish@127.0.0.1:${DEV_DB_PORT:-55432}/atish?sslmode=disable}"
export REDIS_URL="${REDIS_URL:-redis://127.0.0.1:${DEV_REDIS_PORT:-56379}/0}"
export MEDIA_DIR="${MEDIA_DIR:-./data/media}"
cd backend
exec go run ./cmd/cli "$@"
