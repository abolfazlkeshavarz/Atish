#!/usr/bin/env bash
#
# Makes the HOST's nginx the reverse proxy + TLS terminator for Atish and
# obtains its Let's Encrypt certificate.
#
# Why host-level: the "web" container serves plain HTTP on a loopback-only
# port (WEB_BIND/WEB_PORT in .env). One server can host several projects on
# their own (sub)domains, but only one process can own port 443, so nginx lives
# on the host and each project gets its own vhost. This script only writes the
# file named after THIS project's domain — it never edits another project's.
#
#   make ssl                  HTTP-01 challenge (default), auto-renews
#   make ssl CHALLENGE=dns    DNS-01 with a manual TXT record (no auto-renew)
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ "$(id -u)" != "0" ]]; then
  echo "==> Root access is required to configure nginx/certbot; re-running with sudo"
  exec sudo -E bash "$0" "$@"
fi

command -v apt-get >/dev/null 2>&1 || { echo "Ubuntu/Debian (apt) only." >&2; exit 1; }

# shellcheck disable=SC1091
source scripts/lib.sh
load_env .env || exit 1

DOMAIN="${APP_DOMAIN:-${DOMAIN:-}}"
: "${DOMAIN:?Set APP_DOMAIN (or DOMAIN) in .env}"
: "${LETSENCRYPT_EMAIL:?Set LETSENCRYPT_EMAIL in .env}"
APP_HTTP_PORT="${WEB_PORT:-8081}"

CHALLENGE="${CHALLENGE:-http}"
[[ "$CHALLENGE" == "http" || "$CHALLENGE" == "dns" ]] || { echo "CHALLENGE must be http or dns" >&2; exit 1; }

STAGING_FLAG=()
if [[ "${LETSENCRYPT_STAGING:-false}" == "true" ]]; then
  echo "Let's Encrypt staging mode: the certificate will NOT be trusted."
  STAGING_FLAG=(--staging)
fi

VHOST_PATH="/etc/nginx/sites-available/${DOMAIN}.conf"
VHOST_LINK="/etc/nginx/sites-enabled/${DOMAIN}.conf"
CERT_PATH="/etc/letsencrypt/live/${DOMAIN}"
WEBROOT="/var/www/certbot"

echo "==> Installing nginx/certbot if needed (existing installs are left alone)"
apt-get update
apt-get install -y --no-install-recommends nginx certbot gettext-base

[[ -d /etc/nginx/sites-enabled ]] || { echo "Expected the Debian/Ubuntu nginx sites-enabled layout." >&2; exit 1; }
systemctl enable --now nginx
mkdir -p "$WEBROOT"

echo "==> Installing the shared certbot renewal hook"
mkdir -p /etc/letsencrypt/renewal-hooks/deploy
cat > /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh <<'EOF'
#!/bin/sh
nginx -t && systemctl reload nginx
EOF
chmod +x /etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh

if [[ "$CHALLENGE" == "http" ]]; then
  echo "==> Checking that ${DOMAIN} points to this server"
  server_ip="$(curl -fsS --max-time 10 https://api.ipify.org || true)"
  domain_ip="$(getent hosts "${DOMAIN}" | awk '{print $1}' | head -1 || true)"
  if [[ -n "$server_ip" && -n "$domain_ip" && "$server_ip" != "$domain_ip" ]]; then
    echo "Warning: ${DOMAIN} resolves to ${domain_ip}, this server is ${server_ip}."
    read -r -p "Continue anyway? [y/N] " reply
    [[ "$reply" =~ ^[Yy]$ ]] || exit 1
  fi
fi

if [[ -d "$CERT_PATH" ]]; then
  echo "==> Certificate for ${DOMAIN} already exists — re-rendering the vhost only"
elif [[ "$CHALLENGE" == "http" ]]; then
  echo "==> Writing a temporary HTTP-only vhost for the ACME challenge"
  cat > "$VHOST_PATH" <<EOF
server {
    listen 80;
    listen [::]:80;
    server_name ${DOMAIN};
    location ^~ /.well-known/acme-challenge/ { root ${WEBROOT}; default_type "text/plain"; }
    location / { proxy_pass http://127.0.0.1:${APP_HTTP_PORT}; proxy_set_header Host \$host; }
}
EOF
  ln -sf "$VHOST_PATH" "$VHOST_LINK"
  nginx -t && systemctl reload nginx

  echo "==> Requesting the certificate (HTTP-01)"
  certbot certonly --webroot -w "$WEBROOT" "${STAGING_FLAG[@]}" \
    --email "${LETSENCRYPT_EMAIL}" -d "${DOMAIN}" \
    --agree-tos --no-eff-email --non-interactive
else
  echo "==> Requesting the certificate (DNS-01, manual): add the TXT record certbot prints"
  certbot certonly --manual --preferred-challenges dns "${STAGING_FLAG[@]}" \
    --email "${LETSENCRYPT_EMAIL}" -d "${DOMAIN}" --agree-tos --no-eff-email
fi

echo "==> Writing the HTTPS vhost from deploy/nginx/app.conf.template"
DOMAIN="$DOMAIN" APP_HTTP_PORT="$APP_HTTP_PORT" \
  envsubst '${DOMAIN} ${APP_HTTP_PORT}' < deploy/nginx/app.conf.template > "$VHOST_PATH"
ln -sf "$VHOST_PATH" "$VHOST_LINK"
nginx -t
systemctl reload nginx

echo ""
echo "Done: https://${DOMAIN} -> nginx -> 127.0.0.1:${APP_HTTP_PORT} (the Atish web container)."
echo "Renewal is automatic through certbot's systemd timer."
