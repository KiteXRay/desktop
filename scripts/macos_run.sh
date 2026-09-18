#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

mkdir -p "${ROOT_DIR}/build/bin"

echo "==> Compiling kite-tunnel helper..."
go build -o "${ROOT_DIR}/build/bin/kite-tunnel" "${ROOT_DIR}/cmd/kite-tunnel"

echo "==> Ensuring root permissions on kite-tunnel..."
sudo chown root:wheel "${ROOT_DIR}/build/bin/kite-tunnel"
sudo chmod 4755 "${ROOT_DIR}/build/bin/kite-tunnel"

echo "==> Building Kite desktop..."
WAILS_BIN="$(command -v wails3 || command -v "$HOME/go/bin/wails3" || echo "wails3")"
if command -v "$WAILS_BIN" >/dev/null 2>&1; then
    "$WAILS_BIN" task darwin:build
elif command -v task >/dev/null 2>&1; then
    task darwin:build
else
    wails3 task darwin:build
fi

if [ -d "${ROOT_DIR}/build/bin/Kite.app/Contents/MacOS" ]; then
    cp -f "${ROOT_DIR}/build/bin/kite-tunnel" "${ROOT_DIR}/build/bin/Kite.app/Contents/MacOS/kite-tunnel"
    sudo chown root:wheel "${ROOT_DIR}/build/bin/Kite.app/Contents/MacOS/kite-tunnel"
    sudo chmod 4755 "${ROOT_DIR}/build/bin/Kite.app/Contents/MacOS/kite-tunnel"
fi

echo "==> Launching Kite..."
exec "${ROOT_DIR}/build/bin/Kite.app/Contents/MacOS/Kite" "$@"
