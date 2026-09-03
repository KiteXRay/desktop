import type { ConnectionDTO, StatsDTO, AppInfoDTO, ConnectionStatusEvent, ProxyEndpointsDTO, InstalledApp } from '../types';

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
          ParseLinkPreview(link: string): Promise<Record<string, string>>;
          GetTunnelMode(): Promise<string>;
          SetTunnelMode(mode: string): Promise<void>;
          GetProxyEndpoints(): Promise<ProxyEndpointsDTO>;
          GetInstalledApps(): Promise<InstalledApp[]>;
          SetSystemProxy(enabled: boolean): Promise<void>;
          GetSystemProxyStatus(): Promise<boolean>;
          LaunchAppWithProxy(appName: string, targetPath: string): Promise<void>;
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
      version: '1.0.0',
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
  }
};
