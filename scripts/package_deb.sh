#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

VERSION="${1:-1.0.1}"
VERSION="${VERSION#v}" # remove leading 'v'
ARCH="${2:-amd64}"

KITE_BIN="$ROOT_DIR/build/bin/kite"
if [ ! -f "$KITE_BIN" ]; then
    echo "Error: $KITE_BIN not found. Run 'wails build -tags webkit2_41' first." >&2
    exit 1
fi

BUILD_DIR="/tmp/kite-deb-build-$VERSION"
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR/DEBIAN"
mkdir -p "$BUILD_DIR/opt/kite"
mkdir -p "$BUILD_DIR/usr/bin"
mkdir -p "$BUILD_DIR/usr/share/applications"
mkdir -p "$BUILD_DIR/usr/share/icons/hicolor/512x512/apps"
mkdir -p "$BUILD_DIR/usr/share/pixmaps"

# 1. Copy binary & icon & grant script to /opt/kite
cp "$KITE_BIN" "$BUILD_DIR/opt/kite/kite"
chmod 755 "$BUILD_DIR/opt/kite/kite"
cp "$ROOT_DIR/scripts/grant_privileges.sh" "$BUILD_DIR/opt/kite/grant_privileges.sh"
chmod 755 "$BUILD_DIR/opt/kite/grant_privileges.sh"
cp "$ROOT_DIR/build/appicon.png" "$BUILD_DIR/opt/kite/kite.png"
chmod 644 "$BUILD_DIR/opt/kite/kite.png"

# 2. Copy icons for desktop environment
cp "$ROOT_DIR/build/appicon.png" "$BUILD_DIR/usr/share/icons/hicolor/512x512/apps/kite.png"
cp "$ROOT_DIR/build/appicon.png" "$BUILD_DIR/usr/share/pixmaps/kite.png"

# 3. Create CLI symlink
ln -sf /opt/kite/kite "$BUILD_DIR/usr/bin/kite"

# 4. Create desktop file
cat << 'DESKTOP_EOF' > "$BUILD_DIR/usr/share/applications/kite.desktop"
[Desktop Entry]
Name=Kite
Comment=Fast, minimal, and transparent desktop VPN client
Exec=/opt/kite/kite %U
Icon=kite
Terminal=false
Type=Application
Categories=Network;VPN;Security;
StartupWMClass=kite
MimeType=x-scheme-handler/vless;x-scheme-handler/vmess;x-scheme-handler/trojan;x-scheme-handler/ss;
DESKTOP_EOF
chmod 644 "$BUILD_DIR/usr/share/applications/kite.desktop"

# 5. DEBIAN/control
cat << CONTROL_EOF > "$BUILD_DIR/DEBIAN/control"
Package: kite
Version: $VERSION
Section: net
Priority: optional
Architecture: $ARCH
Depends: libgtk-3-0, libwebkit2gtk-4.1-0 | libwebkit2gtk-4.0-37, libcap2-bin
Maintainer: KiteXRay <https://github.com/KiteXRay/desktop>
Description: Fast, minimal, and transparent desktop VPN client
 Kite provides transparent system-wide and per-application tunneling
 powered by Xray-core with dedicated soft TUN routing.
CONTROL_EOF

# 6. DEBIAN/postinst (assigns capabilities and updates caches)
cat << 'POSTINST_EOF' > "$BUILD_DIR/DEBIAN/postinst"
#!/bin/sh
set -e

if command -v setcap >/dev/null 2>&1; then
    setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip /opt/kite/kite 2>/dev/null || true
fi

if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database -q /usr/share/applications 2>/dev/null || true
fi

if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -q /usr/share/icons/hicolor 2>/dev/null || true
fi

exit 0
POSTINST_EOF
chmod 755 "$BUILD_DIR/DEBIAN/postinst"

# 7. DEBIAN/postrm
cat << 'POSTRM_EOF' > "$BUILD_DIR/DEBIAN/postrm"
#!/bin/sh
set -e

if [ "$1" = "remove" ] || [ "$1" = "purge" ]; then
    rm -f /usr/bin/kite
    rm -rf /opt/kite
    if command -v update-desktop-database >/dev/null 2>&1; then
        update-desktop-database -q /usr/share/applications 2>/dev/null || true
    fi
fi

exit 0
POSTRM_EOF
chmod 755 "$BUILD_DIR/DEBIAN/postrm"

# 8. Build deb package
mkdir -p "$ROOT_DIR/build/bin"
OUT_DEB="$ROOT_DIR/build/bin/kite_${VERSION}_${ARCH}.deb"
dpkg-deb -b "$BUILD_DIR" "$OUT_DEB"
rm -rf "$BUILD_DIR"

echo "✓ Created Debian package: $OUT_DEB"
