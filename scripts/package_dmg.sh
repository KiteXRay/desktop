#!/usr/bin/env bash
set -e

# Usage: ./scripts/package_dmg.sh [version] [arch]
VERSION="${1:-1.3.1}"
VERSION="${VERSION#v}"
ARCH="${2:-universal}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BIN_DIR="${ROOT_DIR}/build/bin"
DMG_NAME="kite-macos-${ARCH}.dmg"
OUTPUT_DMG="${BIN_DIR}/${DMG_NAME}"

echo "==> Preparing to build macOS DMG package (${VERSION}, ${ARCH})..."

# Locate the .app bundle
APP_PATH=""
if [ -d "${BIN_DIR}/Kite.app" ]; then
    APP_PATH="${BIN_DIR}/Kite.app"
elif [ -d "${BIN_DIR}/kite.app" ]; then
    # Standardize to Kite.app
    mv "${BIN_DIR}/kite.app" "${BIN_DIR}/Kite.app"
    APP_PATH="${BIN_DIR}/Kite.app"
else
    # Find any .app in build/bin
    FOUND_APP=$(find "${BIN_DIR}" -maxdepth 1 -name "*.app" 2>/dev/null | head -n 1)
    if [ -n "$FOUND_APP" ] && [ -d "$FOUND_APP" ]; then
        if [ "$(basename "$FOUND_APP")" != "Kite.app" ]; then
            mv "$FOUND_APP" "${BIN_DIR}/Kite.app"
        fi
        APP_PATH="${BIN_DIR}/Kite.app"
    fi
fi

if [ -z "$APP_PATH" ] || [ ! -d "$APP_PATH" ]; then
    echo "Error: Could not find Kite.app in ${BIN_DIR}." >&2
    echo "Please build the application first (e.g., 'wails build -platform darwin/universal')." >&2
    exit 1
fi

echo "Found application bundle: ${APP_PATH}"

# Ensure proper icons exist in Contents/Resources
RESOURCES_DIR="${APP_PATH}/Contents/Resources"
mkdir -p "${RESOURCES_DIR}"
if [ -f "${ROOT_DIR}/build/darwin/iconfile.icns" ]; then
    cp "${ROOT_DIR}/build/darwin/iconfile.icns" "${RESOURCES_DIR}/iconfile.icns"
    cp "${ROOT_DIR}/build/darwin/iconfile.icns" "${RESOURCES_DIR}/iconfile"
    cp "${ROOT_DIR}/build/darwin/iconfile.icns" "${RESOURCES_DIR}/iconfile.icns.icns"
    cp "${ROOT_DIR}/build/darwin/iconfile.icns" "${RESOURCES_DIR}/icon.icns"
    cp "${ROOT_DIR}/build/darwin/iconfile.icns" "${RESOURCES_DIR}/appicon.icns"
fi
touch "${APP_PATH}"

# Ensure executable has execute permissions and proper bundle permissions
chmod -R 755 "${APP_PATH}"

TUNNEL_BIN="${BIN_DIR}/kite-tunnel"
if [ ! -f "${TUNNEL_BIN}" ]; then
    echo "==> Compiling kite-tunnel helper for macOS..."
    go build -ldflags "-s -w" -o "${TUNNEL_BIN}" "${ROOT_DIR}/cmd/kite-tunnel"
fi
if [ -f "${TUNNEL_BIN}" ]; then
    cp "${TUNNEL_BIN}" "${APP_PATH}/Contents/MacOS/kite-tunnel"
    chmod 755 "${APP_PATH}/Contents/MacOS/kite-tunnel"
fi

if [ -d "${APP_PATH}/Contents/MacOS" ]; then
    chmod +x "${APP_PATH}/Contents/MacOS"/* || true
fi

# Re-sign the application bundle cleanly so that all resources are sealed with the bundle identifier
if command -v codesign >/dev/null 2>&1; then
    echo "==> Re-signing ${APP_PATH} with ad-hoc signature (identifier: com.kite.desktop)..."
    codesign --force --deep -i com.kite.desktop --sign - "${APP_PATH}" || true
    echo "==> Verifying code signature..."
    codesign --verify --verbose=2 "${APP_PATH}" || true
fi

# Prepare staging directory for DMG
DMG_STAGING="${ROOT_DIR}/build/dmg_staging"
rm -rf "${DMG_STAGING}" "${OUTPUT_DMG}"
mkdir -p "${DMG_STAGING}"

# Copy Kite.app into staging (ditto preserves extended attributes, resource forks, and signatures)
if command -v ditto >/dev/null 2>&1; then
    ditto "${APP_PATH}" "${DMG_STAGING}/Kite.app"
else
    cp -R "${APP_PATH}" "${DMG_STAGING}/"
fi

# Explicitly ensure executable and bundle permissions in staging
chmod -R 755 "${DMG_STAGING}/Kite.app"
if [ -d "${DMG_STAGING}/Kite.app/Contents/MacOS" ]; then
    chmod +x "${DMG_STAGING}/Kite.app/Contents/MacOS"/* || true
fi
touch "${DMG_STAGING}/Kite.app"

# Add standard drag-and-drop link to /Applications
ln -s /Applications "${DMG_STAGING}/Applications"

# Set volume icon if available
if [ -f "${ROOT_DIR}/build/darwin/iconfile.icns" ]; then
    cp "${ROOT_DIR}/build/darwin/iconfile.icns" "${DMG_STAGING}/.VolumeIcon.icns"
fi
if command -v SetFile >/dev/null 2>&1; then
    SetFile -a C "${DMG_STAGING}" || true
fi

echo "==> Generating disk image ${OUTPUT_DMG}..."

# Build DMG using create-dmg, hdiutil, or genisoimage
if command -v create-dmg >/dev/null 2>&1; then
    echo "Using create-dmg utility..."
    CREATE_DMG_ARGS=(
        --volname "Kite"
        --window-pos 200 120
        --window-size 600 400
        --icon-size 100
        --icon "Kite.app" 175 190
        --hide-extension "Kite.app"
        --app-drop-link 425 190
        --no-internet-enable
    )
    if [ -f "${ROOT_DIR}/build/darwin/iconfile.icns" ]; then
        CREATE_DMG_ARGS+=(--volicon "${ROOT_DIR}/build/darwin/iconfile.icns")
    fi

    set +e
    create-dmg "${CREATE_DMG_ARGS[@]}" "${OUTPUT_DMG}" "${DMG_STAGING}"
    STATUS=$?
    set -e

    # create-dmg returns 2 on minor warnings (e.g., missing custom background image), verify if DMG was created
    if [ ! -f "${OUTPUT_DMG}" ]; then
        echo "create-dmg failed (code $STATUS), falling back to hdiutil..."
        if command -v hdiutil >/dev/null 2>&1; then
            hdiutil create -volname "Kite" -srcfolder "${DMG_STAGING}" -ov -format UDZO "${OUTPUT_DMG}"
        fi
    fi
elif command -v hdiutil >/dev/null 2>&1; then
    echo "Using native macOS hdiutil..."
    hdiutil create -volname "Kite" -srcfolder "${DMG_STAGING}" -ov -format UDZO "${OUTPUT_DMG}"
elif command -v genisoimage >/dev/null 2>&1; then
    echo "Using genisoimage (Linux compatibility)..."
    genisoimage -V "Kite" -D -R -apple -no-pad -o "${OUTPUT_DMG}" "${DMG_STAGING}"
else
    echo "Error: Neither create-dmg, hdiutil, nor genisoimage is installed." >&2
    rm -rf "${DMG_STAGING}"
    exit 1
fi

rm -rf "${DMG_STAGING}"

if [ -f "${OUTPUT_DMG}" ]; then
    echo "✓ Successfully created DMG: ${OUTPUT_DMG}"
    ls -lh "${OUTPUT_DMG}"
else
    echo "Error: Failed to create ${OUTPUT_DMG}" >&2
    exit 1
fi
