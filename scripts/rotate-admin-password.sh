#!/usr/bin/env bash
#
# Generates a new admin-panel password, stores it in .env and restarts the API
# so it takes effect. Existing admin sessions stay valid until their token
# expires; rotate JWT_SECRET as well if you need to cut them off immediately.
#
#   make admin-password                     random password
#   make admin-password PASSWORD='...'      choose your own (>= 12 chars)
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck disable=SC1091
source scripts/lib.sh

[[ -f .env ]] || { echo ".env not found — run: make env" >&2; exit 1; }

new="${PASSWORD:-$(rand_secret 18)}"
if [[ ${#new} -lt 12 ]]; then
  echo "The password must be at least 12 characters." >&2
  exit 1
fi

set_env_value .env ADMIN_PASSWORD "$new"
load_env .env

echo "==> Restarting the API with the new password"
"${DOCKER:-docker}" compose up -d backend >/dev/null

echo ""
echo "Admin login:  ${ADMIN_USERNAME:-admin} / ${new}"
echo "(stored in .env — keep that file private)"
