#!/usr/bin/env bash
# Builds kea and installs it as a systemd service (kea.service) that runs
# `kea serve` (the web server, SPA included) on boot.
#
# Usage: sudo ./scripts/install-systemd.sh
#
# Idempotent: safe to re-run after pulling new code to rebuild + redeploy.

set -euo pipefail

if [[ $EUID -ne 0 ]]; then
	echo "This script must be run as root (it creates a system user and installs a systemd unit)." >&2
	echo "Try: sudo $0" >&2
	exit 1
fi

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SERVICE_USER="kea"
SERVICE_HOME="/var/lib/kea"
BIN_PATH="/usr/local/bin/kea"
UNIT_PATH="/etc/systemd/system/kea.service"
BUILD_USER="${SUDO_USER:-}"

echo "==> Building kea (spa + binary)"
if [[ -n "$BUILD_USER" ]]; then
	# Build as the invoking user so `go`/`npm` from their shell profile
	# (not root's minimal sudo PATH) are used.
	sudo -u "$BUILD_USER" -H bash -lc "make -C '$REPO_DIR' build-all"
else
	make -C "$REPO_DIR" build-all
fi

echo "==> Installing binary to ${BIN_PATH}"
install -m 0755 "$REPO_DIR/kea" "$BIN_PATH"

if ! id "$SERVICE_USER" &>/dev/null; then
	echo "==> Creating system user '${SERVICE_USER}'"
	useradd --system --create-home --home-dir "$SERVICE_HOME" --shell /usr/sbin/nologin "$SERVICE_USER"
else
	echo "==> System user '${SERVICE_USER}' already exists, skipping creation"
fi

echo "==> Installing systemd unit to ${UNIT_PATH}"
install -m 0644 "$REPO_DIR/scripts/kea.service" "$UNIT_PATH"

echo "==> Reloading systemd and enabling kea.service"
systemctl daemon-reload
systemctl enable --now kea

echo "==> Done. Status:"
systemctl --no-pager status kea || true
echo
echo "Data dir: ${SERVICE_HOME}/.config/kea (config.yaml, ledgers.yaml, SQLite files)"
echo "Logs:     journalctl -u kea -f"
