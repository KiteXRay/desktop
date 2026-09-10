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
wails build

if [ -d "${ROOT_DIR}/build/bin/Kite.app/Contents/MacOS" ]; then
    cp -f "${ROOT_DIR}/build/bin/kite-tunnel" "${ROOT_DIR}/build/bin/Kite.app/Contents/MacOS/kite-tunnel"
    sudo chown root:wheel "${ROOT_DIR}/build/bin/Kite.app/Contents/MacOS/kite-tunnel"
    sudo chmod 4755 "${ROOT_DIR}/build/bin/Kite.app/Contents/MacOS/kite-tunnel"
fi

echo "==> Launching Kite..."
exec "${ROOT_DIR}/build/bin/Kite.app/Contents/MacOS/Kite" "$@"
