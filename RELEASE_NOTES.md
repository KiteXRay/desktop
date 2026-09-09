## Kite v1.4.1

### 🛠 Fixes & Improvements
- **Bridge Mode in Standalone Helper**: Resolved an issue where Bridge mode behaved identically to standard Tunnel mode when using `kite-tunnel`. Added `BridgeDialer` integration with physical interface bypass binding and dynamic configuration monitoring.
- **WireGuard / AmneziaWG Bridge Compatibility**: Routed WireGuard and AmneziaWG profiles through local netstack SOCKS5 proxy in Bridge mode to enable granular per-process routing rules.
- **Linux Socket Process Resolution**: Improved process resolution for incoming connections by resolving `/proc/[pid]/exe` to prevent 15-character truncation on Linux.
- **Streamlined Privileges Dialog**: Simplified the network privileges modal with concise messaging and removed extraneous error boxes.
