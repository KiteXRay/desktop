import type { ConnectionDTO, StatsDTO, AppInfoDTO, ConnectionStatusEvent, ProxyEndpointsDTO, InstalledApp, ReleaseInfo, UpdateProgress, NetworkPrivilegesDTO, PingResultDTO, Subscription, BridgeGroup, BridgeRule, AddResultDTO, TunnelSettingsDTO } from '../types';

declare global {
  interface Window {
    go?: {
      main?: {
        App?: {
          GetConnections(): Promise<ConnectionDTO[]>;
          AddConnection(label: string, link: string): Promise<ConnectionDTO>;
          UpdateConnection(id: string, label: string, link: string): Promise<void>;
          DeleteConnection(id: string): Promise<void>;
          SwapConnections(id1: number, id2: number): Promise<void>;
          ReorderConnections(from: number, to: number): Promise<void>;
          Connect(id: string): Promise<void>;
          Disconnect(): Promise<void>;
          ClearStuckTun(): Promise<void>;
          Quit(): Promise<void>;
          ResetTraffic(id: string): Promise<void>;
          GetStats(id: string): Promise<StatsDTO>;
          GetAppInfo(): Promise<AppInfoDTO>;
          OpenURL(url: string): Promise<void>;
          GetClipboardText(): Promise<string>;
          ParseLinkPreview(link: string): Promise<Record<string, string>>;
          GetTunnelMode(): Promise<string>;
          SetTunnelMode(mode: string): Promise<void>;
          GetTunnelSettings(): Promise<TunnelSettingsDTO>;
          SetTunnelSettings(deviceIP: string, dns: string): Promise<void>;
          AddConnectionOrSubscription(input: string, label: string): Promise<import('../types').AddResultDTO>;
          GetSubscriptions(): Promise<import('../types').Subscription[]>;
          UpdateSubscription(id: string): Promise<void>;
          DeleteSubscription(id: string): Promise<void>;
          UpdateAllSubscriptions(): Promise<void>;
          GetBridgeGroups(): Promise<import('../types').BridgeGroup[]>;
          SaveBridgeGroups(groups: import('../types').BridgeGroup[]): Promise<void>;
          GetBridgeRules(): Promise<import('../types').BridgeRule[]>;
          SaveBridgeRules(rules: import('../types').BridgeRule[]): Promise<void>;
          CheckRunningBridgeProcesses(): Promise<Record<string, number>>;
          LaunchBridgeRule(ruleID: string, exePath: string): Promise<void>;
          GetProxyEndpoints(): Promise<ProxyEndpointsDTO>;
          GetInstalledApps(): Promise<InstalledApp[]>;
          SetSystemProxy(enabled: boolean): Promise<void>;
          GetSystemProxyStatus(): Promise<boolean>;
          LaunchAppWithProxy(appName: string, targetPath: string): Promise<void>;
          CheckForUpdate(): Promise<ReleaseInfo>;
          InstallUpdate(assetUrl: string, releaseUrl: string): Promise<void>;
          CheckNetworkPrivileges(): Promise<NetworkPrivilegesDTO>;
          GrantNetworkPrivileges(): Promise<boolean>;
          PingConnection(id: string): Promise<number>;
          PingAll(): Promise<Record<string, number>>;
        };
      };
    };
    runtime?: {
      EventsOn(eventName: string, callback: (...args: any[]) => void): () => void;
      EventsOff(eventName: string, ...additionalEvents: string[]): void;
      BrowserOpenURL(url: string): void;
      WindowMinimise(): void;
      WindowToggleMaximise(): void;
      WindowClose(): void;
    };
  }
}

const getApp = () => window.go?.main?.App;

export const api = {
  async getConnections(): Promise<ConnectionDTO[]> {
    const app = getApp();
    if (app) return app.GetConnections();
    return [];
  },

  async addConnection(label: string, link: string): Promise<ConnectionDTO | null> {
    const app = getApp();
    if (app) return app.AddConnection(label, link);
    return null;
  },

  async updateConnection(id: string, label: string, link: string): Promise<void> {
    const app = getApp();
    if (app) return app.UpdateConnection(id, label, link);
  },

  async deleteConnection(id: string): Promise<void> {
    const app = getApp();
    if (app) return app.DeleteConnection(id);
  },

  async swapConnections(id1: number, id2: number): Promise<void> {
    const app = getApp();
    if (app) return app.SwapConnections(id1, id2);
  },

  async reorderConnections(from: number, to: number): Promise<void> {
    const app = getApp();
    if (app?.ReorderConnections) return app.ReorderConnections(from, to);
  },

  async connect(id: string): Promise<void> {
    const app = getApp();
    if (app) return app.Connect(id);
  },

  async disconnect(): Promise<void> {
    const app = getApp();
    if (app) return app.Disconnect();
  },

  async clearStuckTun(): Promise<void> {
    const app = getApp();
    if (app) return app.ClearStuckTun();
  },

  async quit(): Promise<void> {
    const app = getApp();
    if (app) return app.Quit();
  },

  async resetTraffic(id: string): Promise<void> {
    const app = getApp();
    if (app?.ResetTraffic) return app.ResetTraffic(id);
  },

  async getStats(id: string): Promise<StatsDTO> {
    const app = getApp();
    if (app) return app.GetStats(id);
    return {
      id,
      active: false,
      bytesRead: 0,
      bytesWritten: 0,
      uploadSpeed: 0,
      downloadSpeed: 0,
      readHistory: [],
      writeHistory: []
    };
  },

  async getAppInfo(): Promise<AppInfoDTO> {
    const app = getApp();
    if (app) return app.GetAppInfo();
    return {
      name: 'Kite',
      version: '1.2.0',
      repoUrl: 'https://github.com/KiteXRay/desktop',
      os: 'linux',
      arch: 'amd64',
      description: 'Desktop VPN client for Kite'
    };
  },

  async openURL(url: string): Promise<void> {
    const app = getApp();
    if (app) {
      return app.OpenURL(url);
    }
    window.open(url, '_blank');
  },

  async getClipboardText(): Promise<string> {
    const app = getApp();
    if (app?.GetClipboardText) {
      try {
        const text = await app.GetClipboardText();
        if (text) return text;
      } catch {
        // Fallback below
      }
    }
    try {
      return await navigator.clipboard.readText();
    } catch {
      return '';
    }
  },

  async parseLinkPreview(link: string): Promise<Record<string, string>> {
    const app = getApp();
    if (app) return app.ParseLinkPreview(link);
    return {};
  },

  async buildLinkFromConfig(cfg: Record<string, string>): Promise<string> {
    const app = getApp() as any;
    if (app?.BuildLinkFromConfig) return app.BuildLinkFromConfig(cfg);
    return '';
  },

  async getTunnelMode(): Promise<string> {
    const app = getApp();
    if (app?.GetTunnelMode) return app.GetTunnelMode();
    return 'system';
  },

  async setTunnelMode(mode: string): Promise<void> {
    const app = getApp();
    if (app?.SetTunnelMode) return app.SetTunnelMode(mode);
  },

  async getTunnelSettings(): Promise<TunnelSettingsDTO> {
    const app = getApp();
    if (app?.GetTunnelSettings) {
      try {
        const res: any = await app.GetTunnelSettings();
        if (res && typeof res === 'object') {
          const devIP = res.deviceIP || res.deviceIp || res.DeviceIP || (Array.isArray(res) ? res[0] : undefined);
          const dnsVal = res.dns || res.DNS || (Array.isArray(res) ? res[1] : undefined);
          return {
            deviceIP: (typeof devIP === 'string' && devIP.trim()) ? devIP.trim() : '192.18.0.1',
            dns: (typeof dnsVal === 'string' && dnsVal.trim()) ? dnsVal.trim() : '8.8.8.8',
          };
        }
        if (typeof res === 'string' && res.trim()) {
          return { deviceIP: res.trim(), dns: '8.8.8.8' };
        }
      } catch (err) {
        console.error('Failed to get tunnel settings:', err);
      }
    }
    return { deviceIP: '192.18.0.1', dns: '8.8.8.8' };
  },

  async setTunnelSettings(deviceIP: string, dns: string): Promise<void> {
    const app = getApp();
    if (app?.SetTunnelSettings) return app.SetTunnelSettings(deviceIP, dns);
  },

  async addConnectionOrSubscription(input: string, label: string = ''): Promise<AddResultDTO> {
    const app = getApp();
    if (app?.AddConnectionOrSubscription) {
      return app.AddConnectionOrSubscription(input, label);
    }
    // Fallback
    const conn = await app?.AddConnection(label || 'Server', input);
    return { type: 'connection', connection: conn };
  },

  async getSubscriptions(): Promise<Subscription[]> {
    const app = getApp();
    if (app?.GetSubscriptions) return app.GetSubscriptions();
    return [];
  },

  async updateSubscription(id: string): Promise<void> {
    const app = getApp();
    if (app?.UpdateSubscription) return app.UpdateSubscription(id);
  },

  async deleteSubscription(id: string): Promise<void> {
    const app = getApp();
    if (app?.DeleteSubscription) return app.DeleteSubscription(id);
  },

  async updateAllSubscriptions(): Promise<void> {
    const app = getApp();
    if (app?.UpdateAllSubscriptions) return app.UpdateAllSubscriptions();
  },

  async getBridgeGroups(): Promise<BridgeGroup[]> {
    const app = getApp();
    if (app?.GetBridgeGroups) return app.GetBridgeGroups();
    return [];
  },

  async saveBridgeGroups(groups: BridgeGroup[]): Promise<void> {
    const app = getApp();
    if (app?.SaveBridgeGroups) return app.SaveBridgeGroups(groups);
  },

  async getBridgeRules(): Promise<BridgeRule[]> {
    const app = getApp();
    if (app?.GetBridgeRules) return app.GetBridgeRules();
    return [];
  },

  async saveBridgeRules(rules: BridgeRule[]): Promise<void> {
    const app = getApp();
    if (app?.SaveBridgeRules) return app.SaveBridgeRules(rules);
  },

  async checkRunningBridgeProcesses(): Promise<Record<string, number>> {
    const app = getApp();
    if (app?.CheckRunningBridgeProcesses) return app.CheckRunningBridgeProcesses();
    return {};
  },

  async launchBridgeRule(ruleID: string, exePath: string = ''): Promise<void> {
    const app = getApp();
    if (app?.LaunchBridgeRule) return app.LaunchBridgeRule(ruleID, exePath);
  },

  async getProxyEndpoints(): Promise<ProxyEndpointsDTO> {
    const app = getApp();
    if (app?.GetProxyEndpoints) return app.GetProxyEndpoints();
    return {
      socks5Host: '127.0.0.1',
      socks5Port: 10808,
      httpHost: '127.0.0.1',
      httpPort: 10809,
      socks5Url: 'socks5://127.0.0.1:10808',
      httpUrl: 'http://127.0.0.1:10809'
    };
  },

  async setSystemProxy(enabled: boolean): Promise<void> {
    const app = getApp();
    if (app?.SetSystemProxy) return app.SetSystemProxy(enabled);
  },

  async getSystemProxyStatus(): Promise<boolean> {
    const app = getApp();
    if (app?.GetSystemProxyStatus) return app.GetSystemProxyStatus();
    return false;
  },

  async launchAppWithProxy(appName: string, targetPath: string = ''): Promise<void> {
    const app = getApp();
    if (app?.LaunchAppWithProxy) return app.LaunchAppWithProxy(appName, targetPath);
  },

  async getInstalledApps(): Promise<InstalledApp[]> {
    const app = getApp() as any;
    if (app?.GetInstalledApps) {
      const list = await app.GetInstalledApps();
      return list || [];
    }
    return [];
  },

  async selectExecutableDialog(): Promise<string> {
    const app = getApp() as any;
    if (app?.SelectExecutableDialog) return app.SelectExecutableDialog();
    return '';
  },

  async launchAndRouteApp(connectionID: string | number, exePath: string): Promise<void> {
    const app = getApp() as any;
    const idStr = String(connectionID);
    if (app?.LaunchAndRouteApp) return app.LaunchAndRouteApp(idStr, exePath);
    if (app?.LaunchAppWithProxy) return app.LaunchAppWithProxy('', exePath);
  },

  onModeChanged(callback: (mode: string) => void): () => void {
    if (window.runtime?.EventsOn) {
      return window.runtime.EventsOn('mode:changed', callback);
    }
    return () => {};
  },

  onTunnelSettingsChanged(callback: (settings: TunnelSettingsDTO) => void): () => void {
    if (window.runtime?.EventsOn) {
      return window.runtime.EventsOn('tunnel:settings_changed', callback);
    }
    return () => {};
  },

  onStatsTick(callback: (stats: StatsDTO) => void): () => void {
    if (window.runtime?.EventsOn) {
      return window.runtime.EventsOn('stats:tick', callback);
    }
    return () => {};
  },

  onConnectionsChanged(callback: (connections: ConnectionDTO[]) => void): () => void {
    if (window.runtime?.EventsOn) {
      return window.runtime.EventsOn('connections:changed', callback);
    }
    return () => {};
  },

  onConnectionStatus(callback: (event: ConnectionStatusEvent) => void): () => void {
    if (window.runtime?.EventsOn) {
      return window.runtime.EventsOn('connection:status', callback);
    }
    return () => {};
  },

  onProxyStatusChanged(callback: (active: boolean) => void): () => void {
    if (window.runtime?.EventsOn) {
      return window.runtime.EventsOn('proxy:status', callback);
    }
    return () => {};
  },

  onBridgeGroupsChanged(callback: (groups: BridgeGroup[]) => void): () => void {
    if (window.runtime?.EventsOn) {
      return window.runtime.EventsOn('bridge:groups_changed', callback);
    }
    return () => {};
  },

  onBridgeRulesChanged(callback: (rules: BridgeRule[]) => void): () => void {
    if (window.runtime?.EventsOn) {
      return window.runtime.EventsOn('bridge:rules_changed', callback);
    }
    return () => {};
  },

  async checkForUpdate(): Promise<ReleaseInfo | null> {
    const app = getApp() as any;
    if (app?.CheckForUpdate) {
      return app.CheckForUpdate();
    }
    return null;
  },

  async installUpdate(assetUrl: string, releaseUrl: string): Promise<void> {
    const app = getApp() as any;
    if (app?.InstallUpdate) {
      return app.InstallUpdate(assetUrl, releaseUrl);
    }
    window.open(releaseUrl, '_blank');
  },

  async cancelUpdate(): Promise<void> {
    const app = getApp() as any;
    if (app?.CancelUpdate) {
      return app.CancelUpdate();
    }
  },

  onUpdateProgress(callback: (progress: UpdateProgress) => void): () => void {
    if (window.runtime?.EventsOn) {
      return window.runtime.EventsOn('update:progress', callback);
    }
    return () => {};
  },

  async checkNetworkPrivileges(): Promise<NetworkPrivilegesDTO | null> {
    const app = getApp() as any;
    if (app?.CheckNetworkPrivileges) {
      return app.CheckNetworkPrivileges();
    }
    return null;
  },

  async grantNetworkPrivileges(): Promise<boolean> {
    const app = getApp() as any;
    if (app?.GrantNetworkPrivileges) {
      return app.GrantNetworkPrivileges();
    }
    return false;
  },

  onNetworkPrivilegesRequired(callback: (data: { error: string; command: string }) => void): () => void {
    if (window.runtime?.EventsOn) {
      return window.runtime.EventsOn('network:privileges_required', callback);
    }
    return () => {};
  },

  async pingConnection(id: string): Promise<number> {
    const app = getApp() as any;
    if (app?.PingConnection) {
      return app.PingConnection(id);
    }
    return -1;
  },

  async pingAll(): Promise<Record<string, number>> {
    const app = getApp() as any;
    if (app?.PingAll) {
      return app.PingAll();
    }
    return {};
  },

  onPingResult(callback: (res: PingResultDTO) => void): () => void {
    if (window.runtime?.EventsOn) {
      return window.runtime.EventsOn('ping:result', callback);
    }
    return () => {};
  },

  onPingStart(callback: (id: string) => void): () => void {
    if (window.runtime?.EventsOn) {
      return window.runtime.EventsOn('ping:start', callback);
    }
    return () => {};
  }
};
