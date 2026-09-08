## Kite v1.3.2

### 🚀 Improvements & Fixes
- **Accurate Active Latency Probing**: Replaced immediate local SOCKS handshake dial with warm HTTP HEAD probes to `generate_204`, resolving incorrect `ping=1ms` on active VLESS and normalizing active WireGuard/AmneziaWG latency measurements.
- **Physical ISP Pre-Bypass for Profile Pings**: Added `--bypass-ips` support to `kite-tunnel`. When a VPN session starts, all saved profile endpoints are pre-routed through the physical ISP gateway. Pinging any profile while connected now reflects real direct latency without traffic chaining through the active tunnel.
- **Eliminated Routing Loops & Flaps**: Eliminated socket contention and route flapping during latency tests by isolating all profile endpoints at tunnel startup.
