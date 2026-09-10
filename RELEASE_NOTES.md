## Kite v1.4.2

### 🛠 Fixes & Improvements
- **macOS Bridge Mode Direct Bypass**: Fixed an issue on macOS where non-bridged applications were blocked from reaching the internet in Bridge mode. Added Darwin interface-scoped routes (`-ifscope`) and physical interface socket binding (`IP_BOUND_IF`) ensuring non-bridged traffic routes directly through the physical gateway without colliding with TUN routes.
- **macOS Connection & Disconnection Latency**: Optimized proxy state management and concurrent `networksetup` execution on macOS, eliminating the 20–30s delay on connect/disconnect and restoring immediate network responsiveness.
- **macOS Privilege Elevation**: Added `NSAppleEventsUsageDescription` and resolved helper binary symlinks to resolve "Operation not permitted" authorization errors on modern macOS, with automatic terminal fallback for administrative elevation.
- **Tunnel Startup Robustness**: Fixed route parsing, nil gateway pointer safety, and startup race conditions in `kite-tunnel` to prevent premature exits during initialization.
