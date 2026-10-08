#!/usr/bin/env bash
#
# Builds the two images that need compiling (backend + frontend) HERE, on your
# own machine, and packs them into one tarball to carry to the server — so a
# small VPS (1 core / 1 GB) never has to run the Go and Vite builds, which are
# slow at best and OOM-killed at worst. postgres/redis come from Docker Hub.
#
#   make images-bundle
#
# Options (environment variables):
#   PLATFORM=linux/arm64     target architecture if the server is not x86-64
#   VERSION=1.2.0            image tag (default: latest)
#   OUT=path/to/file.tar.gz  where to write the bundle
set -euo pipefail

cd "$(dirname "$0")/.."

PLATFORM="${PLATFORM:-linux/amd64}"
VERSION="${VERSION:-latest}"
OUT="${OUT:-dist/atish-images.tar.gz}"
IMAGES=("atish-backend:${VERSION}" "atish-frontend:${VERSION}")

echo "==> Building images for ${PLATFORM}"
DOCKER_DEFAULT_PLATFORM="$PLATFORM" docker build -t "atish-backend:${VERSION}" ./backend
DOCKER_DEFAULT_PLATFORM="$PLATFORM" docker build -t "atish-frontend:${VERSION}" ./frontend

echo ""
echo "==> Verifying the images really are ${PLATFORM}"
# A wrong architecture loads fine and then dies at start with "exec format error".
want="${PLATFORM##*/}"
for img in "${IMAGES[@]}"; do
  got="$(docker image inspect "$img" --format '{{.Architecture}}')"
  if [[ "$got" != "$want" ]]; then
    echo "Error: ${img} is ${got}, expected ${want}. Check your Docker buildx setup." >&2
    exit 1
  fi
  echo "    ${img}: ${got}"
done

echo ""
echo "==> Packing into ${OUT}"
mkdir -p "$(dirname "$OUT")"
docker save "${IMAGES[@]}" | gzip -1 > "$OUT"

echo ""
echo "Built $(du -h "$OUT" | cut -f1) -> ${OUT}"
echo ""
echo "Next:"
echo "  scp ${OUT} USER@SERVER:/opt/atish/"
echo "  ssh USER@SERVER 'cd /opt/atish && make load-images && make up-prebuilt'"
