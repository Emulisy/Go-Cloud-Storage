#!/usr/bin/env bash
set -Eeuo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Run this script as root: sudo bash deploy/setup-server.sh" >&2
  exit 1
fi

DEPLOY_USER="${SUDO_USER:-root}"
APP_DIR="/opt/gocloudstorage"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname -- "${SCRIPT_DIR}")"

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y ca-certificates curl gnupg rsync debian-keyring debian-archive-keyring apt-transport-https

# Docker's official Ubuntu apt repository.
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
. /etc/os-release
cat > /etc/apt/sources.list.d/docker.sources <<EOF
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: ${UBUNTU_CODENAME:-$VERSION_CODENAME}
Components: stable
Architectures: $(dpkg --print-architecture)
Signed-By: /etc/apt/keyrings/docker.asc
EOF

# Caddy's official stable apt repository.
curl -fsSL https://dl.cloudsmith.io/public/caddy/stable/gpg.key \
  | gpg --dearmor --yes -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -fsSL https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt \
  -o /etc/apt/sources.list.d/caddy-stable.list
chmod o+r /usr/share/keyrings/caddy-stable-archive-keyring.gpg
chmod o+r /etc/apt/sources.list.d/caddy-stable.list

apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin caddy
systemctl enable --now docker

if [[ "${DEPLOY_USER}" != "root" ]]; then
  usermod -aG docker "${DEPLOY_USER}"
fi

install -d -m 0755 -o "${DEPLOY_USER}" -g "${DEPLOY_USER}" "${APP_DIR}"
rsync -a --delete \
  --exclude '.git/' \
  --exclude '.env' \
  --chown="${DEPLOY_USER}:${DEPLOY_USER}" \
  "${PROJECT_DIR}/" "${APP_DIR}/"

if [[ ! -e "${APP_DIR}/.env" ]]; then
  install -m 0600 -o "${DEPLOY_USER}" -g "${DEPLOY_USER}" /dev/null "${APP_DIR}/.env"
fi

install -m 0644 "${APP_DIR}/deploy/Caddyfile" /etc/caddy/Caddyfile
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
systemctl enable --now caddy
systemctl reload caddy

echo
echo "Server setup complete."
echo "Fill in ${APP_DIR}/.env, then run bash ${APP_DIR}/deploy/deploy.sh as ${DEPLOY_USER}."
if [[ "${DEPLOY_USER}" != "root" ]]; then
  echo "Log out and back in first so the Docker group membership takes effect."
fi
