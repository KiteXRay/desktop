#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$ROOT_DIR"

echo "==> Building frontend..."
(cd frontend && npm run build)

echo "==> Ensuring Windows resources and Wintun driver are present..."
mkdir -p build/windows build/bin
if [ ! -f build/windows/wintun.dll ]; then
    echo "Downloading official signed wintun.dll..."
    curl -sSL "https://www.wintun.net/builds/wintun-0.14.1.zip" -o /tmp/wintun.zip
    unzip -q -o /tmp/wintun.zip -d /tmp/wintun
    cp /tmp/wintun/wintun/bin/amd64/wintun.dll build/windows/wintun.dll
fi
cp build/windows/wintun.dll build/bin/wintun.dll
cp build/windows/wintun.dll internal/core/network/tun/wintun.dll

if [ ! -f build/windows/icon.ico ]; then
    echo "Generating Windows icon.ico..."
    python3 -c "from PIL import Image; img = Image.open('build/appicon.png'); img.save('build/windows/icon.ico', format='ICO', sizes=[(16,16), (32,32), (48,48), (64,64), (128,128), (256,256)])"
fi

if [ ! -f icon/assets/icon_default.ico ] || [ ! -f icon/assets/icon_active.ico ]; then
    echo "Generating Windows tray icons..."
    python3 -c "from PIL import Image; Image.open('icon/assets/icon_default.png').convert('RGBA').save('icon/assets/icon_default.ico', format='ICO', sizes=[(16,16),(20,20),(24,24),(32,32),(48,48),(64,64)]); Image.open('icon/assets/icon_active.png').convert('RGBA').save('icon/assets/icon_active.ico', format='ICO', sizes=[(16,16),(20,20),(24,24),(32,32),(48,48),(64,64)])"
fi

echo "==> Synchronizing project version..."
"${SCRIPT_DIR}/set_version.sh" "${APP_VERSION:-${AppVersion:-}}"
APP_VERSION="$(tr -d '[:space:]' < "$ROOT_DIR/VERSION")"

echo "==> Cross-compiling for Windows (x64) with Wails (v${APP_VERSION})..."
export CC=x86_64-w64-mingw32-gcc
export CXX=x86_64-w64-mingw32-g++
wails_bin="$(command -v wails3 || command -v "$HOME/go/bin/wails3" || echo "wails3")"

if command -v "$wails_bin" >/dev/null 2>&1; then
    "$wails_bin" task windows:build ARCH=amd64 PRODUCTION=true
elif command -v task >/dev/null 2>&1; then
    task windows:build ARCH=amd64 PRODUCTION=true
else
    wails3 task windows:build ARCH=amd64 PRODUCTION=true
fi

# Ensure both kite.exe and Kite.exe exist
if [ -f build/bin/Kite.exe ] && [ ! -f build/bin/kite.exe ]; then
    cp -f build/bin/Kite.exe build/bin/kite.exe
elif [ -f build/bin/kite.exe ] && [ ! -f build/bin/Kite.exe ]; then
    cp -f build/bin/kite.exe build/bin/Kite.exe
fi

# NSIS Packaging if makensis available or requested
if command -v makensis >/dev/null 2>&1; then
    echo "==> makensis detected: bundling Windows NSIS Installer..."
    if [ -f build/windows/installer/project.nsi ]; then
        makensis -DARG_WAILS_AMD64_BINARY="..\..\bin\kite.exe" -DINFO_PRODUCTVERSION="${APP_VERSION}" build/windows/installer/project.nsi || true
    fi
elif [ "$1" = "--installer" ] || [ "$1" = "-nsis" ]; then
    echo "==> makensis requested but not found in PATH." >&2
fi

echo "==> Build successful! Windows output located in build/bin/:"
ls -lh build/bin/*.exe build/bin/wintun.dll 2>/dev/null || true
