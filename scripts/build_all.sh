#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Resolve and sync version
"${SCRIPT_DIR}/set_version.sh" "${1:-${APP_VERSION:-${AppVersion:-}}}"
APP_VERSION="$(tr -d '[:space:]' < "$ROOT_DIR/VERSION")"

echo "Building Kite v${APP_VERSION}..."

echo "Building for Linux (amd64)..."
wails build -platform linux/amd64 -tags webkit2_41 -ldflags "-X main.appVersion=${APP_VERSION}"

echo "Building for Linux (arm64)..."
wails build -platform linux/arm64 -tags webkit2_41 -ldflags "-X main.appVersion=${APP_VERSION}"

echo "Building for macOS (universal)..."
wails build -platform darwin/universal -ldflags "-X main.appVersion=${APP_VERSION}"

echo "Build complete! Output files are located in build/bin"
