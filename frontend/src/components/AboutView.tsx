import React, { useEffect, useState } from 'react';
import {
  ExternalLink,
  Cpu,
  Activity,
  HelpCircle,
  RefreshCw,
  CheckCircle2,
  Sparkles,
  Download,
  Loader2,
  Globe,
  Network,
  Layers,
  ShieldCheck,
  Zap,
} from 'lucide-react';
import { api } from '../api/wails';
import type { AppInfoDTO, ReleaseInfo, UpdateProgress } from '../types';

interface AboutViewProps {
  onResetTun?: () => void;
  isResettingTun?: boolean;
  onTriggerUpdate?: (info: ReleaseInfo) => void;
  updateInfo?: ReleaseInfo | null;
  updateProgress?: UpdateProgress | null;
}

export const AboutView: React.FC<AboutViewProps> = ({
  onResetTun,
  isResettingTun,
  onTriggerUpdate,
  updateInfo: externalUpdateInfo,
  updateProgress,
}) => {
  const [appInfo, setAppInfo] = useState<AppInfoDTO | null>(null);
  const [localUpdateInfo, setLocalUpdateInfo] = useState<ReleaseInfo | null>(null);
  const [updateStatus, setUpdateStatus] = useState<'idle' | 'checking' | 'up-to-date' | 'available' | 'error'>('idle');

  useEffect(() => {
    api.getAppInfo().then(setAppInfo).catch(() => {});
  }, []);

  useEffect(() => {
    if (externalUpdateInfo) {
      setLocalUpdateInfo(externalUpdateInfo);
      if (externalUpdateInfo.available) {
        setUpdateStatus('available');
      }
    }
  }, [externalUpdateInfo]);

  const activeUpdateInfo = externalUpdateInfo || localUpdateInfo;

  const handleCheckUpdate = async () => {
    setUpdateStatus('checking');
    try {
      const info = await api.checkForUpdate();
      if (info && info.available) {
        setLocalUpdateInfo(info);
        setUpdateStatus('available');
      } else {
        setUpdateStatus('up-to-date');
      }
    } catch (err) {
      console.error('Update check failed:', err);
      setUpdateStatus('error');
    }
  };

  const openExternal = (url: string) => {
    api.openURL(url);
  };

  const isDownloading = updateProgress?.status === 'downloading';
  const isApplying = updateProgress?.status === 'applying';

  return (
    <div className="max-w-2xl mx-auto py-6 px-4 flex flex-col gap-6 animate-in fade-in duration-200">
      {/* Hero card */}
      <div className="bg-slate-900/60 rounded-2xl p-6 border border-slate-800 text-center flex flex-col items-center gap-3 shadow-xl">
        <img
          src="/icon.png"
          alt="Kite"
          className="w-16 h-16 rounded-2xl border border-indigo-500/30 shadow-xl object-cover"
        />
        <div>
          <h2 className="text-xl font-bold text-slate-100 flex items-center justify-center gap-2">
            <span>{appInfo?.name || 'Kite'}</span>
            <span className="text-xs font-mono font-normal text-indigo-400 bg-indigo-500/10 px-2 py-0.5 rounded-md border border-indigo-500/20">
              XRay
            </span>
          </h2>
          <p className="text-xs text-slate-400 mt-1">
            v{appInfo?.version || '1.4.2'} • {appInfo?.os || 'linux'}/{appInfo?.arch || 'amd64'}
          </p>
        </div>
        <p className="text-sm text-slate-300 max-w-md">
          High-performance, lightweight, and modern desktop VPN client powered by XRay core, Sing-Tun, and WinTun.
        </p>

        {/* Update Checker Section */}
        <div className="w-full mt-2 pt-4 border-t border-slate-800/80 flex flex-col items-center gap-3">
          {updateStatus === 'checking' && (
            <div className="flex items-center gap-2 text-xs text-indigo-400 py-1">
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
              <span>Checking for latest release...</span>
            </div>
          )}

          {updateStatus === 'up-to-date' && (
            <div className="flex items-center gap-2 text-xs text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 px-3 py-1.5 rounded-xl">
              <CheckCircle2 className="w-3.5 h-3.5" />
              <span>You're running the latest version (v{appInfo?.version || '1.4.2'})</span>
            </div>
          )}

          {updateStatus === 'available' && activeUpdateInfo && (
            <div className="w-full bg-gradient-to-r from-indigo-950/40 via-purple-950/30 to-slate-900 border border-indigo-500/30 rounded-xl p-4 flex flex-col sm:flex-row items-center justify-between gap-3 text-left">
              <div className="flex items-center gap-3">
                <div className="w-9 h-9 rounded-xl bg-indigo-500/20 text-indigo-400 flex items-center justify-center shrink-0">
                  <Sparkles className="w-5 h-5" />
                </div>
                <div>
                  <div className="text-xs font-semibold text-slate-100 flex items-center gap-2">
                    <span>New version available!</span>
                    <span className="px-1.5 py-0.5 rounded bg-indigo-500/20 text-indigo-300 text-[10px] font-mono font-bold">
                      {activeUpdateInfo.latestVersion}
                    </span>
                  </div>
                  <div className="text-[11px] text-slate-400 mt-0.5 max-w-xs truncate">
                    {activeUpdateInfo.releaseTitle || 'Bug fixes and performance improvements'}
                  </div>
                </div>
              </div>

              <div className="flex items-center gap-2 shrink-0">
                <button
                  type="button"
                  disabled={isDownloading || isApplying}
                  onClick={() => onTriggerUpdate ? onTriggerUpdate(activeUpdateInfo) : api.installUpdate(activeUpdateInfo.assetUrl, activeUpdateInfo.releaseUrl)}
                  className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-xs font-semibold text-white shadow-md shadow-indigo-600/30 transition-all cursor-pointer whitespace-nowrap"
                >
                  {isDownloading || isApplying ? (
                    <>
                      <Loader2 className="w-3.5 h-3.5 animate-spin" />
                      <span>{isDownloading ? 'Downloading...' : 'Installing...'}</span>
                    </>
                  ) : (
                    <>
                      <Download className="w-3.5 h-3.5" />
                      <span>Update to {activeUpdateInfo.latestVersion}</span>
                    </>
                  )}
                </button>
              </div>
            </div>
          )}

          {updateStatus === 'idle' && (
            <button
              type="button"
              onClick={handleCheckUpdate}
              className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-slate-800/80 hover:bg-slate-800 text-xs text-slate-300 hover:text-slate-100 border border-slate-700/60 transition-colors cursor-pointer whitespace-nowrap"
            >
              <RefreshCw className="w-3.5 h-3.5 text-slate-400" />
              <span>Check for Updates</span>
            </button>
          )}

          {updateStatus === 'error' && (
            <div className="flex items-center gap-2">
              <span className="text-xs text-rose-400">Failed to check for updates</span>
              <button
                type="button"
                onClick={handleCheckUpdate}
                className="text-xs text-slate-400 hover:text-slate-200 underline cursor-pointer"
              >
                Retry
              </button>
            </div>
          )}
        </div>

        <div className="flex flex-wrap items-center justify-center gap-3 mt-2">
          <button
            type="button"
            onClick={() => openExternal(appInfo?.repoUrl || 'https://github.com/KiteXRay/desktop')}
            className="flex items-center gap-1.5 px-4 py-2 text-xs font-medium rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 transition-colors border border-slate-700 cursor-pointer whitespace-nowrap"
          >
            <svg className="w-4 h-4 fill-current" viewBox="0 0 24 24">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
            </svg>
            <span>GitHub Repository</span>
            <ExternalLink className="w-3 h-3 ml-0.5 text-slate-400" />
          </button>
          <button
            type="button"
            onClick={() => openExternal('https://github.com/KiteXRay/desktop/issues/new')}
            className="flex items-center gap-1.5 px-4 py-2 text-xs font-medium rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 transition-colors border border-slate-700 cursor-pointer whitespace-nowrap"
          >
            <HelpCircle className="w-4 h-4" />
            <span>Report an Issue</span>
            <ExternalLink className="w-3 h-3 ml-0.5 text-slate-400" />
          </button>
          {onResetTun && (
            <button
              type="button"
              onClick={onResetTun}
              disabled={isResettingTun}
              className="flex items-center gap-1.5 px-4 py-2 text-xs font-medium rounded-xl bg-slate-800 hover:bg-amber-950/40 text-amber-300 transition-colors border border-slate-700 hover:border-amber-500/40 cursor-pointer disabled:opacity-50 whitespace-nowrap"
              title="Clear stuck TUN interface and reset routing rules"
            >
              <span>{isResettingTun ? 'Resetting TUN...' : 'Reset TUN Interface'}</span>
            </button>
          )}
        </div>
      </div>

      {/* Routing Modes Architecture */}
      <div className="bg-slate-900/40 rounded-2xl p-6 border border-slate-800 flex flex-col gap-4">
        <h3 className="text-sm font-semibold uppercase tracking-wider text-slate-300 flex items-center gap-2">
          <Cpu className="w-4 h-4 text-indigo-400" />
          <span>Three Routing Modes</span>
        </h3>

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 flex flex-col gap-1.5">
            <div className="flex items-center gap-2 text-xs font-semibold text-indigo-300">
              <Globe className="w-3.5 h-3.5" />
              <span>System Tunnel</span>
            </div>
            <p className="text-[11px] text-slate-400 leading-relaxed">
              Virtual network adapter (Sing-Tun / WinTun) routing all system IP packets with custom IP and DNS.
            </p>
          </div>

          <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 flex flex-col gap-1.5">
            <div className="flex items-center gap-2 text-xs font-semibold text-indigo-300">
              <Network className="w-3.5 h-3.5" />
              <span>System Proxy</span>
            </div>
            <p className="text-[11px] text-slate-400 leading-relaxed">
              OS-level HTTP and SOCKS5 proxy integration (WinINet / desktop proxy) without TUN driver requirements.
            </p>
          </div>

          <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 flex flex-col gap-1.5">
            <div className="flex items-center gap-2 text-xs font-semibold text-indigo-300">
              <Layers className="w-3.5 h-3.5" />
              <span>Bridge Mode</span>
            </div>
            <p className="text-[11px] text-slate-400 leading-relaxed">
              Per-application routing matching process executables via regex and wildcards with launcher support.
            </p>
          </div>
        </div>
      </div>

      {/* Protocol & Security Support */}
      <div className="bg-slate-900/40 rounded-2xl p-6 border border-slate-800 flex flex-col gap-3">
        <h3 className="text-sm font-semibold uppercase tracking-wider text-slate-300 flex items-center gap-2">
          <Activity className="w-4 h-4 text-emerald-400" />
          <span>Supported Protocols & Transports</span>
        </h3>
        <p className="text-xs text-slate-400">
          Full compatibility with modern XRay protocols, subscriptions, and security layers:
        </p>
        <div className="flex flex-wrap gap-2 text-xs font-mono">
          <span className="px-2.5 py-1 rounded-lg bg-slate-800 border border-slate-700 text-slate-200 whitespace-nowrap">VLESS</span>
          <span className="px-2.5 py-1 rounded-lg bg-slate-800 border border-slate-700 text-slate-200 whitespace-nowrap">VMess</span>
          <span className="px-2.5 py-1 rounded-lg bg-slate-800 border border-slate-700 text-slate-200 whitespace-nowrap">Trojan</span>
          <span className="px-2.5 py-1 rounded-lg bg-slate-800 border border-slate-700 text-slate-200 whitespace-nowrap">Shadowsocks</span>
          <span className="px-2.5 py-1 rounded-lg bg-slate-800 border border-slate-700 text-slate-200 whitespace-nowrap">XTLS REALITY</span>
          <span className="px-2.5 py-1 rounded-lg bg-slate-800 border border-slate-700 text-slate-200 whitespace-nowrap">Vision</span>
          <span className="px-2.5 py-1 rounded-lg bg-slate-800 border border-slate-700 text-slate-200 whitespace-nowrap">gRPC</span>
          <span className="px-2.5 py-1 rounded-lg bg-slate-800 border border-slate-700 text-slate-200 whitespace-nowrap">WebSocket</span>
          <span className="px-2.5 py-1 rounded-lg bg-slate-800 border border-slate-700 text-slate-200 whitespace-nowrap">SIP008 Subscriptions</span>
        </div>
      </div>

      {/* Key Client Features */}
      <div className="bg-slate-900/40 rounded-2xl p-6 border border-slate-800 flex flex-col gap-3">
        <h3 className="text-sm font-semibold uppercase tracking-wider text-slate-300 flex items-center gap-2">
          <Zap className="w-4 h-4 text-amber-400" />
          <span>Desktop Client Features</span>
        </h3>
        <ul className="space-y-2 text-xs text-slate-300">
          <li className="flex items-start gap-2.5">
            <span className="w-1.5 h-1.5 rounded-full bg-indigo-400 mt-1.5 shrink-0" />
            <span><strong>Window Memory:</strong> Automatically persists last window position, dimensions, and maximized state across reboots.</span>
          </li>
          <li className="flex items-start gap-2.5">
            <span className="w-1.5 h-1.5 rounded-full bg-indigo-400 mt-1.5 shrink-0" />
            <span><strong>Responsive Design & Compact Mode:</strong> Narrow view adapts down to 400px; toggle between comfortable and dense single-row (~36px) server lists with double-click to connect.</span>
          </li>
          <li className="flex items-start gap-2.5">
            <span className="w-1.5 h-1.5 rounded-full bg-indigo-400 mt-1.5 shrink-0" />
            <span><strong>Live Diagnostics & Subscriptions:</strong> Real-time 60-second rolling speed graphs, batch latency pinging (<code className="font-mono text-indigo-300 bg-slate-800 px-1 py-0.5 rounded">Ping all</code>), and one-click subscription updates.</span>
          </li>
          <li className="flex items-start gap-2.5">
            <ShieldCheck className="w-4 h-4 text-emerald-400 shrink-0 mt-0.5" />
            <span><strong>Safe Teardown:</strong> Routing exceptions prevent loops; all virtual devices and routes are automatically restored upon exit.</span>
          </li>
        </ul>
      </div>
    </div>
  );
};
