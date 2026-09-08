#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

if command -v iconutil >/dev/null 2>&1 && command -v sips >/dev/null 2>&1; then
    echo "==> Generating native macOS ICNS icon..."
    mkdir -p "${ROOT_DIR}/build/icon.iconset"
    sips -z 16 16     "${ROOT_DIR}/build/appicon.png" --out "${ROOT_DIR}/build/icon.iconset/icon_16x16.png"
    sips -z 32 32     "${ROOT_DIR}/build/appicon.png" --out "${ROOT_DIR}/build/icon.iconset/icon_16x16@2x.png"
    sips -z 32 32     "${ROOT_DIR}/build/appicon.png" --out "${ROOT_DIR}/build/icon.iconset/icon_32x32.png"
    sips -z 64 64     "${ROOT_DIR}/build/appicon.png" --out "${ROOT_DIR}/build/icon.iconset/icon_32x32@2x.png"
    sips -z 128 128   "${ROOT_DIR}/build/appicon.png" --out "${ROOT_DIR}/build/icon.iconset/icon_128x128.png"
    sips -z 256 256   "${ROOT_DIR}/build/appicon.png" --out "${ROOT_DIR}/build/icon.iconset/icon_128x128@2x.png"
    sips -z 256 256   "${ROOT_DIR}/build/appicon.png" --out "${ROOT_DIR}/build/icon.iconset/icon_256x256.png"
    sips -z 512 512   "${ROOT_DIR}/build/appicon.png" --out "${ROOT_DIR}/build/icon.iconset/icon_256x256@2x.png"
    sips -z 512 512   "${ROOT_DIR}/build/appicon.png" --out "${ROOT_DIR}/build/icon.iconset/icon_512x512.png"
    sips -z 1024 1024 "${ROOT_DIR}/build/appicon.png" --out "${ROOT_DIR}/build/icon.iconset/icon_512x512@2x.png"
    iconutil -c icns "${ROOT_DIR}/build/icon.iconset" -o "${ROOT_DIR}/build/darwin/iconfile.icns"
    cp "${ROOT_DIR}/build/darwin/iconfile.icns" "${ROOT_DIR}/build/darwin/icon.icns"
    cp "${ROOT_DIR}/build/darwin/iconfile.icns" "${ROOT_DIR}/build/darwin/icons.icns"
    rm -rf "${ROOT_DIR}/build/icon.iconset"
fi

echo "==> Synchronizing project version..."
"${SCRIPT_DIR}/set_version.sh" "${APP_VERSION:-${AppVersion:-}}"
APP_VERSION="$(tr -d '[:space:]' < "$ROOT_DIR/VERSION")"

wails build -platform darwin/universal -ldflags "-X main.appVersion=${APP_VERSION}" "$@"
"${SCRIPT_DIR}/package_dmg.sh" "${APP_VERSION}" "universal"