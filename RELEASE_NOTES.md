## Kite v1.1.2

### 🎮 Bridge Mode & SOCKS5 Fixes

#### 1. SOCKS5 UDP Relay in Bridge Mode
- **Loopback Interface Separation**: Resolved an issue where UDP sockets in Bridge mode were bound to the physical network interface, causing the OS to drop UDP packets destined for the local SOCKS5 proxy (`127.0.0.1:10808`). Fixes match connection timeouts in games like *The Finals* (`Discovery.exe`).
- **Anti-Cheat Process Identification**: Added fallback to `CreateToolhelp32Snapshot` on Windows to reliably resolve process names even when anti-cheat drivers (e.g., Easy Anti-Cheat) restrict process handle creation.
- **Ephemeral Port Lookup Backoff**: Added retry backoff for socket table lookups to ensure newly bound UDP ephemeral ports are detected reliably.

## Kite v1.1.1

### 🚀 Self-Update Architecture & Stability Fixes

#### 1. In-App Updater Enhancements
- **Linux Distribution Compatibility**: Fixed asset selection scoring so non-Debian distributions (Arch, Fedora, openSUSE, Alpine, Void) properly select `.tar.gz` packages rather than attempting Debian `.deb` installation.
- **Automated macOS Update**: Full automated self-update on macOS. `.zip` archives are now extracted and `/Applications/Kite.app` is updated directly (with admin escalation via AppleScript when needed) followed by automatic app relaunch.
- **Linux Capability Preservation**: Re-applies network capabilities (`cap_net_raw,cap_net_admin,cap_net_bind_service+eip`) upon executable replacement to ensure TUN device setup remains uninterrupted.
- **Security & Integrity**:
  - SHA256 checksum verification against published release checksums before executing update payloads.
  - Archive extraction hardened against directory traversal (anti-Tar/Zip-Slip) and decompression bombs.
  - Temporary files and extraction directories are now created with strict permissions and cleaned up automatically.
- **SemVer 2.0 Engine**: Upgraded version comparison to standard SemVer 2.0 with pre-release tag support, ensuring beta testers correctly receive stable release updates.
- **GitHub API Rate-Limit Resilience**: Added `If-None-Match` (ETag) caching to avoid 60 req/hr unauthenticated rate-limit errors.
- **Compile-Time Version Injection**: Release workflow now dynamically injects version tags into the binary via `-ldflags`.

#### 2. User Experience & Controls
- **Download Cancellation**: Added a "Cancel Download" button in the update modal so in-progress downloads can be safely aborted.
- **Snooze & Skip Controls**: Added "Remind Tomorrow" (24-hour snooze) and "Skip Version" options to prevent intrusive modal alerts on every startup.

