#!/usr/bin/env bash
#
# Zero-to-deployed on a fresh Ubuntu/Debian VPS: installs Docker, writes .env
# with generated secrets, builds (or loads) the images, starts the stack, then
# configures the host's nginx + a Let's Encrypt certificate.
#
# Runs happily next to OTHER projects on the same server: only this project's
# containers are touched, and TLS reuses the host's single nginx/certbot (see
# scripts/deploy-host-nginx.sh) instead of fighting for port 443.
#
# Usage (project root, after git clone):
#   ./scripts/bootstrap-vps.sh
# Non-interactive:
#   DOMAIN=atish.example.com LETSENCRYPT_EMAIL=me@example.com \
#   TELEGRAM_BOT_TOKEN=123:abc TELEGRAM_BOT_USERNAME=AtishBot \
#   ADMIN_TELEGRAM_IDS=123456789 ./scripts/bootstrap-vps.sh
#
# Another project already uses 8081? Pick the local port yourself:
#   WEB_PORT=8082 ./scripts/bootstrap-vps.sh
set -euo pipefail

cd "$(dirname "$0")/.."
# shellcheck disable=SC1091
source scripts/lib.sh

if [[ "$(id -u)" != "0" ]]; then
  echo "==> Root access is required to install Docker; re-running with sudo"
  exec sudo -E bash "$0" "$@"
fi

REAL_USER="${SUDO_USER:-root}"

if ! command -v apt-get >/dev/null 2>&1; then
  echo "This script is written only for Ubuntu/Debian (apt)." >&2
  exit 1
fi

# --------------------------------------------------------- base packages
echo "==> Installing base packages"
apt-get update
apt-get install -y --no-install-recommends ca-certificates curl gnupg make openssl git

# --------------------------------------------------------------- Docker Engine
if ! command -v docker >/dev/null 2>&1; then
  echo "==> Installing Docker Engine"
  install -m 0755 -d /etc/apt/keyrings
  # shellcheck disable=SC1091
  . /etc/os-release
  distro="${ID:-ubuntu}"; [[ "$distro" == "debian" ]] || distro="ubuntu"
  curl -fsSL "https://download.docker.com/linux/${distro}/gpg" -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/${distro} ${VERSION_CODENAME} stable" \
    > /etc/apt/sources.list.d/docker.list
  apt-get update
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  systemctl enable --now docker
else
  echo "==> Docker is already installed"
fi

if [[ "$REAL_USER" != "root" ]] && ! id -nG "$REAL_USER" | grep -qw docker; then
  echo "==> Adding $REAL_USER to the docker group (log out/in to use docker without sudo)"
  usermod -aG docker "$REAL_USER"
fi

# --------------------------------------------------------------- domain and email
[[ -z "${DOMAIN:-}" ]] && read -r -p "Domain whose A record points to this server: " DOMAIN
[[ -z "${LETSENCRYPT_EMAIL:-}" ]] && read -r -p "Email for Let's Encrypt expiry warnings: " LETSENCRYPT_EMAIL
: "${DOMAIN:?DOMAIN is required}"
: "${LETSENCRYPT_EMAIL:?LETSENCRYPT_EMAIL is required}"

if [[ -z "${WEB_PORT:-}" ]]; then
  WEB_PORT="$(find_free_port 8081)"
  if [[ -d /etc/nginx || -x /usr/sbin/nginx ]]; then
    echo ""
    echo "nginx is already on this server (another project is probably deployed)."
    echo "Atish will listen privately on 127.0.0.1:${WEB_PORT} (auto-picked as free)."
    read -r -p "Local port for Atish [${WEB_PORT}]: " reply
    WEB_PORT="${reply:-$WEB_PORT}"
  fi
fi

# ------------------------------------------------------------------- .env
if [[ ! -f .env ]]; then
  echo "==> Creating .env with generated secrets"
  DOMAIN="$DOMAIN" LETSENCRYPT_EMAIL="$LETSENCRYPT_EMAIL" bash scripts/init-env.sh
  set_env_value .env WEB_BIND 127.0.0.1
  set_env_value .env WEB_PORT "$WEB_PORT"
  GENERATED_ENV=1
else
  echo "==> .env already exists — leaving it as is"
  load_env .env
fi

[[ "$REAL_USER" != "root" ]] && chown "$REAL_USER":"$REAL_USER" .env
chmod 600 .env
load_env .env

if [[ -z "${TELEGRAM_BOT_TOKEN:-}" ]]; then
  echo ""
  echo "NOTE: TELEGRAM_BOT_TOKEN is empty in .env. The app will start, but nobody can"
  echo "      log in until you add the token from @BotFather and run:  make up"
fi

# --------------------------------------------------------------------- images
VERSION="${VERSION:-latest}"
if docker image inspect "atish-backend:${VERSION}" >/dev/null 2>&1 \
   && docker image inspect "atish-frontend:${VERSION}" >/dev/null 2>&1; then
  echo "==> Prebuilt images found; skipping the build"
  make up-prebuilt
else
  echo "==> Building images (slow on a small server). If it runs out of memory, build on"
  echo "    your own machine instead: make images-bundle, scp it over, make load-images."
  make up
fi

# --------------------------------------------------------------- TLS + nginx
echo "==> Obtaining the SSL certificate and configuring host nginx"
make ssl

echo ""
echo "================================================================"
echo " Atish is live at https://${DOMAIN}"
echo "================================================================"
echo "  Admin panel:  https://${DOMAIN}/admin   (user: ${ADMIN_USERNAME:-admin})"
[[ "${GENERATED_ENV:-0}" == "1" ]] && echo "  Password:     ${ADMIN_PASSWORD}   (also stored in .env)"
echo "  Health check: make deploy-health HOST=https://${DOMAIN}"
echo "  Make yourself an admin inside Telegram:  make admin-promote TG=<your telegram id> PROD=1"
echo "  Finally, in @BotFather set the bot's Menu Button / Mini App URL to https://${DOMAIN}"
