#!/usr/bin/env bash
set -e

wails build -tags webkit2_41
sudo setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip build/bin/kite
exec ./build/bin/kite