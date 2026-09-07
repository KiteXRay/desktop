export interface ConnectionDTO {
  id: string;
  subscriptionId?: string;
  label: string;
  link: string;
  active: boolean;
  address: string;
  port: string;
  protocol: string;
  tls: string;
  flow: string;
  network: string;
  security: string;
  configMap: Record<string, string>;
  bytesRead: number;
  bytesWritten: number;
  totalBytes?: number;
  pingMs?: number;
}

export interface PingResultDTO {
  id: string;
  pingMs: number;
}

export interface StatsDTO {
  id: string;
  active: boolean;
  bytesRead: number;
  bytesWritten: number;
  totalBytes?: number;
  uploadSpeed: number;   // KB/s
  downloadSpeed: number; // KB/s
  readHistory: number[];   // MB
  writeHistory: number[];  // MB
}

export interface AppInfoDTO {
  name: string;
  version: string;
  repoUrl: string;
  os: string;
  arch: string;
  description: string;
}

export interface ConnectionStatusEvent {
  status: 'connected' | 'disconnected' | 'error' | 'reconnecting';
  id: string;
  error?: string;
  mode?: string;
  message?: string;
}

export type TunnelMode = 'tunnel' | 'proxy' | 'bridge' | 'system' | 'per_app';

export interface TunnelSettingsDTO {
  deviceIP: string;
  dns: string;
}

export interface Subscription {
  id: string;
  subId?: string;
  url: string;
  label: string;
  count: number;
  lastUpdated: number;
  userInfo?: string;
}

export interface BridgeRule {
  id: string;
  pattern: string;
  proxyTarget: string;
  proxyType: 'socks5' | 'http';
  enabled: boolean;
  description?: string;
}

export interface AddResultDTO {
  type: 'subscription' | 'connection';
  id?: string;
  label?: string;
  count?: number;
  lastUpdated?: number;
  connection?: ConnectionDTO;
}

export interface ProxyEndpointsDTO {
  socks5Host: string;
  socks5Port: number;
  httpHost: string;
  httpPort: number;
  socks5Url: string;
  httpUrl: string;
}

export interface InstalledApp {
  name: string;
  exePath: string;
  icon?: string;
  description?: string;
}

export interface ReleaseInfo {
  available: boolean;
  currentVersion: string;
  latestVersion: string;
  releaseTitle: string;
  releaseNotes: string;
  releaseUrl: string;
  assetUrl: string;
  assetName: string;
  assetSize: number;
}

export interface UpdateProgress {
  status: 'checking' | 'downloading' | 'applying' | 'completed' | 'error';
  percentage: number;
  downloaded: number;
  total: number;
  error?: string;
}

export interface NetworkPrivilegesDTO {
  hasPrivileges: boolean;
  os: string;
  exePath: string;
  command: string;
  error?: string;
}


