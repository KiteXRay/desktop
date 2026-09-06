## Kite v1.1.0

### 🚀 What's New

#### 1. Operating Modes (Tunnel / Proxy / Bridge)
- **Tunnel Mode**: System-wide VPN tunnel with configurable device IP and custom DNS settings.
- **Proxy Mode**: System-wide HTTP / SOCKS proxy configuration.
- **Bridge Mode**: Rule-based per-application proxying.
  - Process matching with wildcards (`*`, `?`) and regular expressions.
  - Transparent connection interception via OS socket lookup tables (Windows & Linux).
  - Built-in installed applications browser for easy rule creation and rule editing.
- **Mode Settings Dialog**: Dedicated 3-tab configuration modal for all operating modes.

#### 2. Subscription Management & Smart Profile Grouping
- **Subscription Support**: Import subscription links directly into the app alongside standard proxy links (`vless://`, `vmess://`, `trojan://`, `ss://`).
- **Profile Grouping**:
  - Manual links are organized under the **Local** group.
  - Subscriptions are organized into unique **Subscription-{subId}** groups, parsed from subscription URLs or web endpoints.
  - Collapsible group views with quick refresh and delete actions.
- **In-Place Refresh**: Subscriptions update connection parameters in place without losing connection stats or ID, reconnecting automatically if active.

#### 3. UI & Connection Enhancements
- Expanded Reality Flow options (`xtls-rprx-vision`, `xtls-rprx-vision-udp443`), protocols, transports, and XHTTP modes.
- Streamlined profile cards and details view.
- Faster, more resilient connection handling across all platforms.
