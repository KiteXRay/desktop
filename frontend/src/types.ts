export interface ConnectionDTO {
  id: string;
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

export type TunnelMode = 'system' | 'per_app';

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


