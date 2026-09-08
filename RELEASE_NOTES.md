## Kite v1.3.1

### 🚀 Improvements & Fixes
- **Native Direct TUN for WireGuard & AmneziaWG**: Replaced double TCP/IP stack (`tun2socks` + loopback SOCKS5) with direct `amneziawg-go` device bound straight to the OS TUN interface (`kite0`). Eliminates packet aliasing, latency, and CPU overhead in tunnel mode.
- **Fixed Endless Ping & CPU Spikes on Active Connections**: Replaced redundant in-memory WireGuard device creation during active session pings with direct warm latency probing through the active tunnel, stopping session hijacking and handshake loops.
- **Fixed `wireguard://` Link Parsing**: Enabled native WireGuard URI query parameter extraction (`address`, `publickey`, `keepalive`, `mtu`, etc.) and safe JSON type decoding.
- **Fixed UDP Relay Packet Dropping**: Cloned socket address and packet slices in SOCKS5 UDP associate relay to avoid symmetric NAT packet drops.
- **Upgraded Connectivity Probes**: Updated tunnel verification targets to standard global HTTPS and DNS endpoints (`cp.cloudflare.com:80`, `1.1.1.1:443`).

