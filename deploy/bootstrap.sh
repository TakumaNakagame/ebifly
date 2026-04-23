#!/usr/bin/env bash
# One-shot installer for an Ubuntu 24.04 cloud-init VM.
# Installs Docker, Docker Compose plugin, and sets up the ebifly project dir.
# Idempotent — safe to re-run.

set -euo pipefail

PROJECT_DIR=${PROJECT_DIR:-/opt/ebifly}

echo "==> apt update / upgrade"
sudo apt-get update -y
sudo DEBIAN_FRONTEND=noninteractive apt-get upgrade -y

echo "==> installing Docker"
if ! command -v docker >/dev/null 2>&1; then
  curl -fsSL https://get.docker.com | sudo sh
  sudo usermod -aG docker "$(whoami)"
fi

echo "==> creating project dir ${PROJECT_DIR}"
sudo mkdir -p "${PROJECT_DIR}"
sudo chown "$(whoami):$(whoami)" "${PROJECT_DIR}"

echo "==> done"
echo "Next:"
echo "  1. scp the ebifly source (or checkout) into ${PROJECT_DIR}"
echo "  2. Create ${PROJECT_DIR}/.env with:"
echo "       CLOUDFLARE_TUNNEL_TOKEN=<from terraform output>"
echo "       ALLOWED_ORIGINS=ebifly.cf.kameneko.dev"
echo "  3. cd ${PROJECT_DIR} && docker compose -f docker-compose.tunnel.yml up -d --build"
