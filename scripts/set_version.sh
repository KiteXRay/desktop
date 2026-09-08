#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Resolution priority: $1 -> $APP_VERSION -> $AppVersion -> $(cat VERSION) -> 1.3.2
TARGET_VERSION="${1:-${APP_VERSION:-${AppVersion:-}}}"

if [ -z "$TARGET_VERSION" ]; then
    if [ -f "$ROOT_DIR/VERSION" ]; then
        TARGET_VERSION="$(tr -d '[:space:]' < "$ROOT_DIR/VERSION")"
    else
        TARGET_VERSION="1.3.2"
    fi
fi

# Strip leading 'v'
TARGET_VERSION="${TARGET_VERSION#v}"

if [ -z "$TARGET_VERSION" ]; then
    echo "Error: Version cannot be empty." >&2
    exit 1
fi

python3 - "$ROOT_DIR" "$TARGET_VERSION" << 'PYEOF'
import sys
import os
import json
import re

root_dir = sys.argv[1]
version = sys.argv[2].strip()

# Parse SemVer (major.minor.patch)
parts = version.split(".")
try:
    major = int(parts[0])
    minor = int(parts[1]) if len(parts) > 1 else 0
    # patch may have suffix like 2-beta
    patch_str = parts[2] if len(parts) > 2 else "0"
    patch_match = re.match(r'^(\d+)', patch_str)
    patch = int(patch_match.group(1)) if patch_match else 0
except ValueError:
    major, minor, patch = 1, 0, 0

build = 0
quad_version = f"{major}.{minor}.{patch}.{build}"

# 1. Update VERSION file
version_file = os.path.join(root_dir, "VERSION")
with open(version_file, "w", encoding="utf-8") as f:
    f.write(f"{version}\n")

# 2. Update wails.json
wails_file = os.path.join(root_dir, "wails.json")
if os.path.isfile(wails_file):
    with open(wails_file, "r", encoding="utf-8") as f:
        wails_data = json.load(f)
    if "info" not in wails_data:
        wails_data["info"] = {}
    wails_data["info"]["productVersion"] = version
    with open(wails_file, "w", encoding="utf-8") as f:
        json.dump(wails_data, f, indent=2)
        f.write("\n")

# 3. Update build/windows/info.json
win_info_file = os.path.join(root_dir, "build", "windows", "info.json")
if os.path.isfile(win_info_file):
    with open(win_info_file, "r", encoding="utf-8") as f:
        win_data = json.load(f)
    
    fixed = win_data.get("FixedFileInfo", {})
    fixed["FileVersion"] = {"Major": major, "Minor": minor, "Patch": patch, "Build": build}
    fixed["ProductVersion"] = {"Major": major, "Minor": minor, "Patch": patch, "Build": build}
    win_data["FixedFileInfo"] = fixed

    string_info = win_data.get("StringFileInfo", {})
    string_info["FileVersion"] = quad_version
    string_info["ProductVersion"] = quad_version
    win_data["StringFileInfo"] = string_info

    with open(win_info_file, "w", encoding="utf-8") as f:
        json.dump(win_data, f, indent=2)
        f.write("\n")

# 4. Update app.go
app_go_file = os.path.join(root_dir, "app.go")
if os.path.isfile(app_go_file):
    with open(app_go_file, "r", encoding="utf-8") as f:
        content = f.read()
    new_content = re.sub(r'var appVersion = "[^"]*"', f'var appVersion = "{version}"', content)
    if new_content != content:
        with open(app_go_file, "w", encoding="utf-8") as f:
            f.write(new_content)

# 5. Update build/windows/installer/kite.iss
iss_file = os.path.join(root_dir, "build", "windows", "installer", "kite.iss")
if os.path.isfile(iss_file):
    with open(iss_file, "r", encoding="utf-8") as f:
        content = f.read()
    new_content = re.sub(r'#define MyAppVersion "[^"]*"', f'#define MyAppVersion "{version}"', content)
    if new_content != content:
        with open(iss_file, "w", encoding="utf-8") as f:
            f.write(new_content)

# 6. Update frontend/package.json
pkg_file = os.path.join(root_dir, "frontend", "package.json")
if os.path.isfile(pkg_file):
    with open(pkg_file, "r", encoding="utf-8") as f:
        pkg_data = json.load(f)
    pkg_data["version"] = version
    with open(pkg_file, "w", encoding="utf-8") as f:
        json.dump(pkg_data, f, indent=2)
        f.write("\n")

# 7. Update frontend/src/api/wails.ts mock fallback
api_wails_file = os.path.join(root_dir, "frontend", "src", "api", "wails.ts")
if os.path.isfile(api_wails_file):
    with open(api_wails_file, "r", encoding="utf-8") as f:
        content = f.read()
    new_content = re.sub(r"(version:\s*')[^']*(',)", f"\\g<1>{version}\\g<2>", content)
    if new_content != content:
        with open(api_wails_file, "w", encoding="utf-8") as f:
            f.write(new_content)

print(f"✓ Synchronized project version to {version} (quad: {quad_version})")
PYEOF
