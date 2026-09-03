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
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
    gtk-update-icon-cache -q /usr/share/icons/hicolor 2>/dev/null || true
fi

echo "✓ Kite has been completely uninstalled."
