#!/usr/bin/env bash
set -e

# Automatically elevate with sudo if not root
if [ "$EUID" -ne 0 ]; then
    echo "==> Administrator (sudo) privileges required to install Kite to /opt/."
    exec sudo bash "$0" "$@"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." 2>/dev/null && pwd || echo "$SCRIPT_DIR")"

# Locate kite binary
KITE_BIN=""
if [ -f "$SCRIPT_DIR/kite" ]; then
    KITE_BIN="$SCRIPT_DIR/kite"
elif [ -f "$SCRIPT_DIR/build/bin/kite" ]; then
    KITE_BIN="$SCRIPT_DIR/build/bin/kite"
elif [ -f "$ROOT_DIR/build/bin/kite" ]; then
    KITE_BIN="$ROOT_DIR/build/bin/kite"
elif [ -f "./kite" ]; then
    KITE_BIN="$(pwd)/kite"
fi

if [ -z "$KITE_BIN" ]; then
    echo "Error: 'kite' executable not found in $SCRIPT_DIR or $ROOT_DIR/build/bin." >&2
    exit 1
fi

echo "==> Installing Kite to /opt/kite/..."
mkdir -p /opt/kite

# Copy binary
cp "$KITE_BIN" /opt/kite/kite
chmod 755 /opt/kite/kite

# Assign network capabilities
echo "==> Assigning network capabilities (CAP_NET_ADMIN, CAP_NET_RAW, CAP_NET_BIND_SERVICE)..."
if command -v setcap >/dev/null 2>&1; then
    setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip /opt/kite/kite
    echo "  ✓ Capabilities successfully assigned to /opt/kite/kite"
else
    echo "  ⚠ Warning: 'setcap' tool not found. Install libcap2-bin (Debian/Ubuntu) or libcap (Arch/Fedora)."
fi

# Locate and install icon
ICON_SRC=""
if [ -f "$SCRIPT_DIR/kite.png" ]; then
    ICON_SRC="$SCRIPT_DIR/kite.png"
elif [ -f "$ROOT_DIR/build/appicon.png" ]; then
    ICON_SRC="$ROOT_DIR/build/appicon.png"
fi

if [ -n "$ICON_SRC" ]; then
    cp "$ICON_SRC" /opt/kite/kite.png
    chmod 644 /opt/kite/kite.png
    mkdir -p /usr/share/icons/hicolor/512x512/apps /usr/share/pixmaps
    cp "$ICON_SRC" /usr/share/icons/hicolor/512x512/apps/kite.png
    cp "$ICON_SRC" /usr/share/pixmaps/kite.png
fi

# Create desktop entry
echo "==> Creating system desktop entry..."
mkdir -p /usr/share/applications
cat << 'DESKTOP_EOF' > /usr/share/applications/kite.desktop
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
chmod 644 /usr/share/applications/kite.desktop

# Create CLI symlink
mkdir -p /usr/local/bin
ln -sf /opt/kite/kite /usr/local/bin/kite

# Create uninstaller
cat << 'UNINSTALL_EOF' > /opt/kite/uninstall.sh
#!/usr/bin/env bash
set -e
if [ "$EUID" -ne 0 ]; then
    echo "Elevating with sudo..."
    exec sudo bash "$0" "$@"
fi
echo "==> Uninstalling Kite..."
rm -f /usr/local/bin/kite
rm -f /usr/share/applications/kite.desktop
rm -f /usr/share/icons/hicolor/512x512/apps/kite.png
rm -f /usr/share/pixmaps/kite.png
rm -rf /opt/kite
if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database -q /usr/share/applications 2>/dev/null || true
fi
echo "✓ Kite has been completely uninstalled."
UNINSTALL_EOF
chmod 755 /opt/kite/uninstall.sh

# Refresh desktop & icon databases
if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database -q /usr/share/applications 2>/dev/null || true
fi
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -q /usr/share/icons/hicolor 2>/dev/null || true
fi

echo ""
echo "================================================================="
echo "  ✓ Kite installed successfully to /opt/kite/kite!"
echo "  ✓ Capabilities assigned: no sudo required to connect VPN"
echo "  ✓ Desktop launcher added to Applications menu"
echo "  ✓ CLI command available: 'kite'"
echo "  ✓ Uninstaller saved at /opt/kite/uninstall.sh"
echo "================================================================="
