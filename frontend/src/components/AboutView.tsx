import React, { useEffect, useState } from 'react';
import { ExternalLink, Cpu, Activity, HelpCircle } from 'lucide-react';
import { api } from '../api/wails';
import type { AppInfoDTO } from '../types';

interface AboutViewProps {
  onResetTun?: () => void;
  isResettingTun?: boolean;
}

export const AboutView: React.FC<AboutViewProps> = ({ onResetTun, isResettingTun }) => {
  const [appInfo, setAppInfo] = useState<AppInfoDTO | null>(null);

  useEffect(() => {
    api.getAppInfo().then(setAppInfo).catch(() => {});
  }, []);

  const openExternal = (url: string) => {
    api.openURL(url);
  };

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
          <h2 className="text-xl font-bold text-slate-100">{appInfo?.name || 'Kite'}</h2>
          <p className="text-xs text-slate-400 mt-1">
            v{appInfo?.version || '1.0.0'} • {appInfo?.os || 'linux'}/{appInfo?.arch || 'amd64'}
          </p>
        </div>
        <p className="text-sm text-slate-300 max-w-md">
          High-performance, lightweight, and modern desktop VPN client powered by XRay core.
        </p>

        <div className="flex flex-wrap items-center justify-center gap-3 mt-2">
          <button
            onClick={() => openExternal(appInfo?.repoUrl || 'https://github.com/KiteXRay/desktop')}
            className="flex items-center gap-1.5 px-4 py-2 text-xs font-medium rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 transition-colors border border-slate-700 cursor-pointer"
          >
            <svg className="w-4 h-4 fill-current" viewBox="0 0 24 24">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
            </svg>
            <span>GitHub Repository</span>
            <ExternalLink className="w-3 h-3 ml-0.5 text-slate-400" />
          </button>
          <button
            onClick={() => openExternal('https://github.com/KiteXRay/desktop/issues/new')}
            className="flex items-center gap-1.5 px-4 py-2 text-xs font-medium rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 transition-colors border border-slate-700 cursor-pointer"
          >
            <HelpCircle className="w-4 h-4" />
            <span>Report an Issue</span>
            <ExternalLink className="w-3 h-3 ml-0.5 text-slate-400" />
          </button>
          {onResetTun && (
            <button
              onClick={onResetTun}
              disabled={isResettingTun}
              className="flex items-center gap-1.5 px-4 py-2 text-xs font-medium rounded-xl bg-slate-800 hover:bg-amber-950/40 text-amber-300 transition-colors border border-slate-700 hover:border-amber-500/40 cursor-pointer disabled:opacity-50"
              title="Clear stuck TUN interface and reset routing rules"
            >
              <span>{isResettingTun ? 'Resetting TUN...' : 'Reset TUN Interface'}</span>
            </button>
          )}
        </div>
      </div>

      {/* Architecture & How it works */}
      <div className="bg-slate-900/40 rounded-2xl p-6 border border-slate-800 flex flex-col gap-4">
        <h3 className="text-sm font-semibold uppercase tracking-wider text-slate-300 flex items-center gap-2">
          <Cpu className="w-4 h-4 text-indigo-400" />
          <span>How It Works</span>
        </h3>

        <ul className="space-y-2.5 text-xs text-slate-300">
          <li className="flex items-start gap-2.5">
            <span className="w-1.5 h-1.5 rounded-full bg-indigo-400 mt-1.5 shrink-0" />
            <span><strong>TUN Device:</strong> The client creates a dedicated virtual network TUN interface to capture packet-level traffic.</span>
          </li>
          <li className="flex items-start gap-2.5">
            <span className="w-1.5 h-1.5 rounded-full bg-indigo-400 mt-1.5 shrink-0" />
            <span><strong>Soft Routing Rules:</strong> Only additional rules are added for the lifetime of the TUN device. Your default system routes remain intact.</span>
          </li>
          <li className="flex items-start gap-2.5">
            <span className="w-1.5 h-1.5 rounded-full bg-indigo-400 mt-1.5 shrink-0" />
            <span><strong>Direct Exception:</strong> An exception route is added for the VPN server's outbound endpoint to prevent routing loops.</span>
          </li>
          <li className="flex items-start gap-2.5">
            <span className="w-1.5 h-1.5 rounded-full bg-indigo-400 mt-1.5 shrink-0" />
            <span><strong>Clean Exit:</strong> When disconnecting or shutting down, all routing rules and virtual devices are automatically torn down and cleaned up.</span>
          </li>
        </ul>
      </div>

      {/* Protocol Support */}
      <div className="bg-slate-900/40 rounded-2xl p-6 border border-slate-800 flex flex-col gap-3">
        <h3 className="text-sm font-semibold uppercase tracking-wider text-slate-300 flex items-center gap-2">
          <Activity className="w-4 h-4 text-emerald-400" />
          <span>Supported Protocols</span>
        </h3>
        <p className="text-xs text-slate-400">
          Supports all modern XRay protocols via URL scheme notations:
        </p>
        <div className="flex flex-wrap gap-2 text-xs font-mono">
          <span className="px-2.5 py-1 rounded-lg bg-slate-800 border border-slate-700 text-slate-200">vless://</span>
          <span className="px-2.5 py-1 rounded-lg bg-slate-800 border border-slate-700 text-slate-200">vmess://</span>
          <span className="px-2.5 py-1 rounded-lg bg-slate-800 border border-slate-700 text-slate-200">trojan://</span>
          <span className="px-2.5 py-1 rounded-lg bg-slate-800 border border-slate-700 text-slate-200">shadowsocks://</span>
          <span className="px-2.5 py-1 rounded-lg bg-slate-800 border border-slate-700 text-slate-200">VLESS + XTLS REALITY</span>
        </div>
      </div>
    </div>
  );
};
