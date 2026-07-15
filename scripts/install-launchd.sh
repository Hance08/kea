#!/usr/bin/env bash
# Builds kea and installs it as a launchd LaunchAgent (com.kea.serve) that
# runs `kea serve` (the web server, SPA included) automatically at login,
# including after a restart, and restarts it if it crashes.
#
# Usage: ./scripts/install-launchd.sh
#
# Idempotent: safe to re-run after pulling new code to rebuild + redeploy.
# macOS only; use scripts/install-systemd.sh on Linux.

set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
	echo "This script is for macOS (launchd). Use scripts/install-systemd.sh on Linux." >&2
	exit 1
fi

if [[ $EUID -eq 0 ]]; then
	echo "Run this as your normal user, not root (LaunchAgents run per-user; sudo is used only to install the binary)." >&2
	exit 1
fi

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_PATH="/usr/local/bin/kea"
LABEL="com.kea.serve"
PLIST_PATH="$HOME/Library/LaunchAgents/${LABEL}.plist"
LOG_DIR="$HOME/Library/Logs/kea"
UID_NUM="$(id -u)"
DOMAIN="gui/${UID_NUM}"

echo "==> Building kea (spa + binary)"
make -C "$REPO_DIR" build-all

echo "==> Installing binary to ${BIN_PATH} (requires sudo)"
sudo install -m 0755 "$REPO_DIR/kea" "$BIN_PATH"

echo "==> Ensuring log directory ${LOG_DIR} exists"
mkdir -p "$LOG_DIR"

echo "==> Generating LaunchAgent plist at ${PLIST_PATH}"
mkdir -p "$HOME/Library/LaunchAgents"
sed \
	-e "s#__BIN_PATH__#${BIN_PATH}#g" \
	-e "s#__LOG_DIR__#${LOG_DIR}#g" \
	-e "s#__HOME_DIR__#${HOME}#g" \
	"$REPO_DIR/scripts/kea.plist.template" >"$PLIST_PATH"

echo "==> (Re)loading ${LABEL} into launchd"
launchctl bootout "$DOMAIN" "$PLIST_PATH" 2>/dev/null || true
launchctl bootstrap "$DOMAIN" "$PLIST_PATH"
launchctl enable "${DOMAIN}/${LABEL}"
launchctl kickstart -k "${DOMAIN}/${LABEL}"

echo "==> Done. Status:"
launchctl print "${DOMAIN}/${LABEL}" 2>/dev/null | head -20 || true
echo
echo "Data dir: ~/.config/kea (config.yaml, ledgers.yaml, SQLite files)"
echo "Logs:     ${LOG_DIR}/kea.log"
echo "Stop:     launchctl bootout ${DOMAIN} ${PLIST_PATH}"
echo "Start:    launchctl bootstrap ${DOMAIN} ${PLIST_PATH}"
