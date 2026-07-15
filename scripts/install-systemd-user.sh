#!/usr/bin/env bash
# Builds kea and installs it as a systemd --user service (kea.service) that
# runs `kea serve` for the current user only, without root or a dedicated
# system account. By default a user service starts at login and stops at
# logout; enable lingering (printed below) to have it start at boot / after
# a restart and keep running without an interactive session.
#
# Usage: ./scripts/install-systemd-user.sh
#
# Idempotent: safe to re-run after pulling new code to rebuild + redeploy.
# Runs as your normal user; use scripts/install-systemd.sh (sudo, system
# service) instead if you want a dedicated service account.

set -euo pipefail

if [[ "$(uname -s)" != "Linux" ]]; then
	echo "This script is for Linux (systemd --user). Use scripts/install-launchd.sh on macOS." >&2
	exit 1
fi

if [[ $EUID -eq 0 ]]; then
	echo "Run this as your normal user, not root (systemd --user services run per-user)." >&2
	exit 1
fi

if ! command -v systemctl &>/dev/null; then
	echo "systemctl not found; this script requires systemd." >&2
	exit 1
fi

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="$HOME/.local/bin"
BIN_PATH="$BIN_DIR/kea"
UNIT_DIR="$HOME/.config/systemd/user"
UNIT_PATH="$UNIT_DIR/kea.service"

echo "==> Building kea (spa + binary)"
make -C "$REPO_DIR" build-all

echo "==> Installing binary to ${BIN_PATH}"
mkdir -p "$BIN_DIR"
install -m 0755 "$REPO_DIR/kea" "$BIN_PATH"

case ":$PATH:" in
*":$BIN_DIR:"*) ;;
*) echo "Note: ${BIN_DIR} is not on your PATH. Add it in your shell profile to run 'kea' directly." ;;
esac

echo "==> Installing user unit to ${UNIT_PATH}"
mkdir -p "$UNIT_DIR"
install -m 0644 "$REPO_DIR/scripts/kea-user.service" "$UNIT_PATH"

echo "==> Reloading systemd --user and enabling kea.service"
systemctl --user daemon-reload
systemctl --user enable --now kea

echo "==> Done. Status:"
systemctl --user --no-pager status kea || true

if ! loginctl show-user "$USER" -p Linger 2>/dev/null | grep -q 'Linger=yes'; then
	echo
	echo "Note: user services normally start at login and stop at logout."
	echo "To have kea start at boot and survive logout, enable lingering:"
	echo "  sudo loginctl enable-linger $USER"
fi

echo
echo "Data dir: ~/.config/kea (config.yaml, ledgers.yaml, SQLite files)"
echo "Logs:     journalctl --user -u kea -f"
