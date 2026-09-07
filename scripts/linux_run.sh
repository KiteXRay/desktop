#!/usr/bin/env bash
set -e

wails build -tags webkit2_41
if [ ! -f build/bin/kite-tunnel ] || [ cmd/kite-tunnel/main.go -nt build/bin/kite-tunnel ]; then
    go build -o build/bin/kite-tunnel ./cmd/kite-tunnel
fi
exec ./build/bin/kite