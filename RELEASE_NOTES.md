## Kite v1.3.0

### 🚀 Features & Enhancements
- **WireGuard & AmneziaWG (AWG) Protocol Support**: Integrated userspace AmneziaWG engine with SOCKS5 bridge and full support for obfuscation parameters (`Jc`, `Jmin`, `Jmax`, `S1`, `S2`, `H1`–`H4`).
- **Configuration Import**: Added direct import for WireGuard and AmneziaWG `.conf` files via file picker dialog and URI pasting.
- **Robust Outbound Probes**: Improved health watchdog and tunnel connectivity verification to avoid DNS deadlock during handshake.
