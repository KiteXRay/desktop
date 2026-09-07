#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
wails build -platform darwin/universal "$@"
"${SCRIPT_DIR}/package_dmg.sh" "1.2.0" "universal"