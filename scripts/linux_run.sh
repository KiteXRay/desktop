#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$ROOT_DIR"

WAILS_BIN="$(command -v wails3 || command -v "$HOME/go/bin/wails3" || echo "wails3")"

if command -v "$WAILS_BIN" >/dev/null 2>&1; then
    "$WAILS_BIN" task build
elif command -v task >/dev/null 2>&1; then
    task build
else
    wails3 task build
fi

if [ ! -f build/bin/kite-tunnel ] || [ cmd/kite-tunnel/main.go -nt build/bin/kite-tunnel ]; then
    go build -o build/bin/kite-tunnel ./cmd/kite-tunnel
fi

if [ -f build/bin/Kite ]; then
    exec ./build/bin/Kite "$@"
else
    exec ./build/bin/kite "$@"
fi