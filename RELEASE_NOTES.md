## Kite v1.2.0

### 🌐 Routing & Bridge Enhancements

#### 1. Collapsible Bridge Rule Groups & Batch Controls
- **Rule Grouping**: Organize split tunneling rules into collapsible groups with status badges and rule counters.
- **Group Master Toggle**: Quickly enable or disable entire groups of rules with a single click.
- **Auto-Assignment**: Automatic categorization of applications and domains into rule groups for cleaner management.
- **State Persistence**: Group assignments and toggle states are saved and restored seamlessly across app restarts.

#### 2. System Tray Routing Mode Switcher
- **Tray Mode Sub-Menu**: Switch between Tunnel (TUN), Proxy, and Bridge (Split Tunneling) modes directly from the system tray menu.
- **Live State Indicators**: Visual checkmarks and indicators in the tray menu display the currently active routing mode in real time.

### ⚡ Connection & Settings Improvements

#### 1. Automatic Ping on Connect
- **Instant Latency Metrics**: Profile ping is now automatically triggered upon connection, showing real-time latency right away without requiring manual refresh.

#### 2. Tunnel DNS & Settings Persistence
- **Reliable DNS Serialization**: Adopted structured `TunnelSettingsDTO` across Go backend and frontend Wails bindings to eliminate issues where custom DNS settings were dropped.
- **Settings Synchronization**: Validated and synced custom tunnel IP and DNS configurations in the Mode Settings modal, ensuring accurate persistence in configuration files.
