#!/usr/bin/env bash
set -e

EXE="${1:-/opt/kite/kite}"
PID="$2"

echo "==> Preparing to grant network capabilities to: $EXE"

# 1. If caller PID is provided, wait for it to exit
if [ -n "$PID" ]; then
    echo "Waiting for process $PID to exit..."
    for i in {1..50}; do
        if ! kill -0 "$PID" 2>/dev/null; then
            break
        fi
        sleep 0.1
    done
fi

# 2. Ensure all instances are closed so file is not busy
echo "Closing any running instances of Kite..."
fuser -k -TERM "$EXE" 2>/dev/null || true
pkill -TERM -f "$EXE" 2>/dev/null || true
sleep 0.3

# 3. Apply capabilities
echo "Applying network capabilities..."
if [ "$EUID" -ne 0 ]; then
    if command -v pkexec >/dev/null 2>&1; then
        pkexec setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip "$EXE"
    else
        sudo setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip "$EXE"
    fi
else
    setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip "$EXE"
fi

echo "✓ Successfully applied network capabilities to $EXE"

# 4. Relaunch
if [ "${3:-relaunch}" = "relaunch" ]; then
    echo "Relaunching Kite..."
    nohup "$EXE" >/dev/null 2>&1 &
fi
