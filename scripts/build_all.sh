#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$ROOT_DIR"

# Resolve and sync version
"${SCRIPT_DIR}/set_version.sh" "${1:-${APP_VERSION:-${AppVersion:-}}}"
APP_VERSION="$(tr -d '[:space:]' < "$ROOT_DIR/VERSION")"

echo "Building Kite v${APP_VERSION}..."

WAILS_BIN="$(command -v wails3 || command -v "$HOME/go/bin/wails3" || echo "wails3")"

run_task() {
    if command -v "$WAILS_BIN" >/dev/null 2>&1; then
        "$WAILS_BIN" task "$@"
    elif command -v task >/dev/null 2>&1; then
        task "$@"
    else
        wails3 task "$@"
    fi
}

echo "Building for Linux (amd64)..."
run_task linux:build ARCH=amd64 PRODUCTION=true
cp -f build/bin/Kite "build/bin/Kite-linux-amd64" 2>/dev/null || true

echo "Building for Linux (arm64)..."
run_task linux:build ARCH=arm64 PRODUCTION=true
cp -f build/bin/Kite "build/bin/Kite-linux-arm64" 2>/dev/null || true

echo "Building for macOS (universal)..."
run_task darwin:build PRODUCTION=true

echo "Building for Windows (amd64)..."
run_task windows:build ARCH=amd64 PRODUCTION=true

echo "Build complete! Output files are located in build/bin"
