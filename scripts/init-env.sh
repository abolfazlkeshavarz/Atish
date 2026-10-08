#!/usr/bin/env bash
#
# Creates .env from .env.example with freshly generated secrets, so a new
# install never ships a placeholder password.
#
#   make env
#   DOMAIN=atish.example.com LETSENCRYPT_EMAIL=me@example.com make env
#
# Never overwrites an existing .env (delete it yourself if you really want a
# new one — rotating ENCRYPTION_KEY makes stored phone numbers unreadable).
set -euo pipefail
cd "$(dirname "$0")/.."
# shellcheck disable=SC1091
source scripts/lib.sh

if [[ -f .env ]]; then
  echo ".env already exists — leaving it untouched."
  exit 0
fi

cp .env.example .env

ADMIN_PASSWORD="${ADMIN_PASSWORD:-$(rand_secret 18)}"

set_env_value .env POSTGRES_PASSWORD "$(rand_secret 24)"
set_env_value .env JWT_SECRET "$(rand_secret 48)"
set_env_value .env ENCRYPTION_KEY "$(openssl rand -base64 32)"
set_env_value .env ADMIN_PASSWORD "$ADMIN_PASSWORD"
set_env_value .env APP_ENV production
set_env_value .env DEV_AUTH false

if [[ -n "${DOMAIN:-}" ]]; then
  set_env_value .env APP_DOMAIN "$DOMAIN"
  set_env_value .env DOMAIN "$DOMAIN"
  set_env_value .env PUBLIC_URL "https://${DOMAIN}"
fi
[[ -n "${LETSENCRYPT_EMAIL:-}" ]] && set_env_value .env LETSENCRYPT_EMAIL "$LETSENCRYPT_EMAIL"
[[ -n "${TELEGRAM_BOT_TOKEN:-}" ]] && set_env_value .env TELEGRAM_BOT_TOKEN "$TELEGRAM_BOT_TOKEN"
[[ -n "${TELEGRAM_BOT_USERNAME:-}" ]] && set_env_value .env TELEGRAM_BOT_USERNAME "$TELEGRAM_BOT_USERNAME"
[[ -n "${ADMIN_TELEGRAM_IDS:-}" ]] && set_env_value .env ADMIN_TELEGRAM_IDS "$ADMIN_TELEGRAM_IDS"

chmod 600 .env 2>/dev/null || true

echo "Created .env with generated secrets."
echo "  Admin panel login:  ${ADMIN_USERNAME:-admin} / ${ADMIN_PASSWORD}"
echo "  Still to fill in:   TELEGRAM_BOT_TOKEN, TELEGRAM_BOT_USERNAME (from @BotFather)"
