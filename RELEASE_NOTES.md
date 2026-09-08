## Kite v1.4.0

### 🚀 AmneziaWG 2.0 & WireGuard Features
- **AmneziaWG 2.0 Protocol Support**: Added full support for AmneziaWG 2.0 parameters, including `H1`–`H4` packet header ranges, `S3`/`S4` handshake padding bounds, and `I1`–`I5` junk packet obfuscation sequence chains.
- **WireGuard & AmneziaWG Profile Editor**: Added manual profile creation and editing in the application modal (`AddEditModal`). Configurable parameters now include `AllowedIPs`, `DNS`, `PersistentKeepalive`, `Address`, `MTU`, `Reserved`, plus all AmneziaWG obfuscation fields (`Jc`, `Jmin`, `Jmax`, `S1`–`S4`, `H1`–`H4`, `I1`–`I5`).
- **Config & URI Serialization**: Added bidirectional conversion and parsing for AWG 2.0 parameters in standard configuration files (`.conf`) and `awg://` share links.

### 🛠 Fixes & Stability
- **Native TUN Gateway Routing**: Fixed point-to-point peer routing gateway assignment for native WireGuard/AWG tunnel interfaces (`kite-tunnel`), resolving unreachable gateway errors on custom interface subnets.
- **Profile Switching & Reconnection Guards**: Prevented background watchdog reconnection loops from hijacking connection state or holding SOCKS ports when switching profiles or disconnecting.
