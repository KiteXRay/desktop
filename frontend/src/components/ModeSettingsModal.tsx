import React, { useState, useEffect } from 'react';
import {
  X,
  Settings,
  Globe,
  Network,
  Layers,
  Check,
  Copy,
  RotateCcw,
  Loader2,
  AlertCircle,
  Terminal,
  ShieldCheck,
} from 'lucide-react';
import { api } from '../api/wails';
import type { ProxyEndpointsDTO } from '../types';
import { BridgeView } from './BridgeView';

interface ModeSettingsModalProps {
  isOpen: boolean;
  initialTab?: 'tunnel' | 'proxy' | 'bridge';
  onClose: () => void;
  onResetTun?: () => Promise<void>;
  isResettingTun?: boolean;
  isConnected?: boolean;
  activeLabel?: string;
  onConnect?: () => void;
  showToast?: (msg: string, type?: 'success' | 'error' | 'info') => void;
}

export const ModeSettingsModal: React.FC<ModeSettingsModalProps> = ({
  isOpen,
  initialTab = 'tunnel',
  onClose,
  onResetTun,
  isResettingTun = false,
  isConnected = false,
  activeLabel,
  onConnect,
  showToast,
}) => {
  const [activeTab, setActiveTab] = useState<'tunnel' | 'proxy' | 'bridge'>('tunnel');

  // Tunnel state
  const [deviceIP, setDeviceIP] = useState('192.18.0.1');
  const [dns, setDNS] = useState('8.8.8.8');
  const [tunnelLoading, setTunnelLoading] = useState(false);
  const [tunnelError, setTunnelError] = useState<string | null>(null);
  const [tunnelSaved, setTunnelSaved] = useState(false);

  // Proxy state
  const [systemProxyEnabled, setSystemProxyEnabled] = useState(false);
  const [proxyLoading, setProxyLoading] = useState(false);
  const [endpoints, setEndpoints] = useState<ProxyEndpointsDTO>({
    socks5Host: '127.0.0.1',
    socks5Port: 10808,
    httpHost: '127.0.0.1',
    httpPort: 10809,
    socks5Url: 'socks5://127.0.0.1:10808',
    httpUrl: 'http://127.0.0.1:10809',
  });
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [cliShell, setCliShell] = useState<'bash' | 'powershell' | 'cmd'>('bash');

  useEffect(() => {
    if (!isOpen) return;
    if (initialTab) {
      setActiveTab(initialTab);
    }

    // Load Tunnel settings
    api.getTunnelSettings()
      .then((settings) => {
        if (settings.deviceIP) setDeviceIP(settings.deviceIP);
        if (settings.dns) setDNS(settings.dns);
      })
      .catch((err) => console.error('Failed to load tunnel settings:', err));

    // Load Proxy settings & endpoints
    api.getSystemProxyStatus()
      .then((status) => setSystemProxyEnabled(status))
      .catch((err) => console.error('Failed to get proxy status:', err));

    api.getProxyEndpoints()
      .then((res) => setEndpoints(res))
      .catch((err) => console.error('Failed to get proxy endpoints:', err));

    setTunnelError(null);
    setTunnelSaved(false);
  }, [isOpen, initialTab]);

  if (!isOpen) return null;

  const handleTunnelSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const cleanIP = deviceIP.trim();
    const cleanDNS = dns.trim();

    if (!cleanIP) {
      setTunnelError('Device IP address is required');
      return;
    }
    if (!cleanDNS) {
      setTunnelError('DNS server is required');
      return;
    }

    const ipPattern = /^(\d{1,3}\.){3}\d{1,3}$/;
    if (!ipPattern.test(cleanIP)) {
      setTunnelError('Invalid Device IP format (expected e.g. 192.18.0.1)');
      return;
    }
    if (!ipPattern.test(cleanDNS)) {
      setTunnelError('Invalid DNS format (expected e.g. 8.8.8.8)');
      return;
    }

    setTunnelError(null);
    setTunnelLoading(true);

    try {
      await api.setTunnelSettings(cleanIP, cleanDNS);
      setTunnelSaved(true);
      showToast?.('Tunnel adapter settings saved', 'success');
      setTimeout(() => setTunnelSaved(false), 2500);
    } catch (err: any) {
      setTunnelError(err?.message || 'Failed to save tunnel settings');
    } finally {
      setTunnelLoading(false);
    }
  };

  const handleToggleSystemProxy = async () => {
    const nextState = !systemProxyEnabled;
    setProxyLoading(true);
    try {
      await api.setSystemProxy(nextState);
      setSystemProxyEnabled(nextState);
      showToast?.(
        nextState ? 'System proxy enabled' : 'System proxy disabled',
        'success'
      );
    } catch (err: any) {
      showToast?.(`Failed to toggle system proxy: ${err?.message || err}`, 'error');
    } finally {
      setProxyLoading(false);
    }
  };

  const copyText = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const dnsPresets = [
    { label: 'Google (8.8.8.8)', value: '8.8.8.8' },
    { label: 'Cloudflare (1.1.1.1)', value: '1.1.1.1' },
    { label: 'Quad9 (9.9.9.9)', value: '9.9.9.9' },
  ];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-xs animate-in fade-in duration-150">
      <div
        className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-4xl shadow-2xl overflow-hidden flex flex-col max-h-[88vh]"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Modal Header with 3 Tabs */}
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 px-6 py-4 border-b border-slate-800 bg-slate-900/90 shrink-0">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400 shrink-0">
              <Settings className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-base font-bold text-slate-100 flex items-center gap-2">
                Routing Mode Settings
              </h2>
              <p className="text-xs text-slate-400 mt-0.5">
                Configure parameters for Tunnel, Proxy, and Bridge modes
              </p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            {/* 3-tab segmented control */}
            <div className="flex items-center bg-slate-950 p-1 rounded-xl border border-slate-800 shrink-0">
              <button
                type="button"
                onClick={() => setActiveTab('tunnel')}
                className={`flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                  activeTab === 'tunnel'
                    ? 'bg-indigo-600 text-white shadow-xs'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <Globe className="w-3.5 h-3.5" />
                <span>Tunnel</span>
              </button>

              <button
                type="button"
                onClick={() => setActiveTab('proxy')}
                className={`flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                  activeTab === 'proxy'
                    ? 'bg-indigo-600 text-white shadow-xs'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <Network className="w-3.5 h-3.5" />
                <span>Proxy</span>
              </button>

              <button
                type="button"
                onClick={() => setActiveTab('bridge')}
                className={`flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                  activeTab === 'bridge'
                    ? 'bg-indigo-600 text-white shadow-xs'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
              >
                <Layers className="w-3.5 h-3.5" />
                <span>Bridge</span>
              </button>
            </div>

            <button
              onClick={onClose}
              className="p-1.5 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors cursor-pointer"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        {/* Modal Body */}
        <div className="flex-1 overflow-y-auto p-6">
          {/* TAB 1: TUNNEL */}
          {activeTab === 'tunnel' && (
            <div className="space-y-6 max-w-2xl mx-auto animate-in fade-in duration-150">
              <div className="p-4 rounded-xl bg-slate-950/60 border border-slate-800">
                <div className="flex items-center gap-2 mb-1.5">
                  <Globe className="w-4 h-4 text-indigo-400" />
                  <h3 className="text-sm font-bold text-slate-100">Virtual TUN Adapter</h3>
                </div>
                <p className="text-xs text-slate-400 leading-relaxed">
                  System Tunnel mode sets up a virtual TUN network adapter (Sing-Tun / WinTun) to route all system IP packets through your active VPN profile.
                </p>
              </div>

              <form onSubmit={handleTunnelSubmit} className="space-y-5">
                {tunnelError && (
                  <div className="p-3 bg-rose-500/10 border border-rose-500/20 rounded-xl flex items-center gap-2.5 text-xs text-rose-300">
                    <AlertCircle className="w-4 h-4 shrink-0" />
                    <span>{tunnelError}</span>
                  </div>
                )}

                {tunnelSaved && (
                  <div className="p-3 bg-emerald-500/10 border border-emerald-500/20 rounded-xl flex items-center gap-2.5 text-xs text-emerald-300">
                    <Check className="w-4 h-4 shrink-0" />
                    <span>Settings saved successfully!</span>
                  </div>
                )}

                {/* Device IP */}
                <div className="space-y-1.5">
                  <label className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                    TUN Device IP Address
                  </label>
                  <input
                    type="text"
                    value={deviceIP}
                    onChange={(e) => setDeviceIP(e.target.value)}
                    placeholder="192.18.0.1"
                    className="w-full px-3.5 py-2 rounded-xl bg-slate-950/70 border border-slate-800 focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 text-slate-100 placeholder-slate-600 text-xs font-mono transition-colors outline-hidden"
                  />
                  <p className="text-[11px] text-slate-500">
                    Virtual IPv4 address assigned to the local TUN interface (default: 192.18.0.1)
                  </p>
                </div>

                {/* DNS */}
                <div className="space-y-1.5">
                  <label className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                    Virtual DNS Server
                  </label>
                  <input
                    type="text"
                    value={dns}
                    onChange={(e) => setDNS(e.target.value)}
                    placeholder="8.8.8.8"
                    className="w-full px-3.5 py-2 rounded-xl bg-slate-950/70 border border-slate-800 focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 text-slate-100 placeholder-slate-600 text-xs font-mono transition-colors outline-hidden"
                  />

                  {/* DNS Presets */}
                  <div className="flex flex-wrap gap-1.5 pt-1">
                    {dnsPresets.map((preset) => (
                      <button
                        key={preset.value}
                        type="button"
                        onClick={() => setDNS(preset.value)}
                        className={`text-[11px] px-2.5 py-1 rounded-lg border transition-all cursor-pointer ${
                          dns === preset.value
                            ? 'bg-indigo-600/20 text-indigo-300 border-indigo-500/40 font-medium'
                            : 'bg-slate-800/40 text-slate-400 hover:text-slate-200 border-slate-700/50 hover:bg-slate-800'
                        }`}
                      >
                        {preset.label}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="flex items-center justify-between pt-2">
                  {onResetTun && (
                    <button
                      type="button"
                      onClick={onResetTun}
                      disabled={isResettingTun}
                      className="flex items-center gap-1.5 px-3 py-1.5 bg-slate-800 hover:bg-amber-950/40 text-slate-300 hover:text-amber-300 border border-slate-700 hover:border-amber-500/40 rounded-xl text-xs font-medium transition-all cursor-pointer"
                      title="Reset any stuck virtual TUN adapters"
                    >
                      {isResettingTun ? (
                        <Loader2 className="w-3.5 h-3.5 animate-spin" />
                      ) : (
                        <RotateCcw className="w-3.5 h-3.5" />
                      )}
                      <span>Reset TUN Adapter</span>
                    </button>
                  )}

                  <button
                    type="submit"
                    disabled={tunnelLoading}
                    className="flex items-center gap-2 px-5 py-2 rounded-xl bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-xs font-semibold transition-all shadow-md shadow-indigo-600/20 cursor-pointer ml-auto"
                  >
                    {tunnelLoading && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                    <span>Save Tunnel Settings</span>
                  </button>
                </div>
              </form>
            </div>
          )}

          {/* TAB 2: PROXY */}
          {activeTab === 'proxy' && (
            <div className="space-y-6 max-w-2xl mx-auto animate-in fade-in duration-150">
              <div className="p-4 rounded-xl bg-slate-950/60 border border-slate-800">
                <div className="flex items-center gap-2 mb-1.5">
                  <Network className="w-4 h-4 text-indigo-400" />
                  <h3 className="text-sm font-bold text-slate-100">System-Wide Proxy</h3>
                </div>
                <p className="text-xs text-slate-400 leading-relaxed">
                  System Proxy configures the operating system&apos;s global proxy settings (WinINet on Windows, desktop proxy on Linux). All web browsers and proxy-aware applications route traffic through Kite without needing TUN driver privileges.
                </p>
              </div>

              {/* System Proxy Toggle Card */}
              <div className="flex items-center justify-between p-4 rounded-2xl bg-slate-900/60 border border-slate-800">
                <div className="flex items-center gap-3">
                  <div className={`w-9 h-9 rounded-xl flex items-center justify-center border transition-all ${
                    systemProxyEnabled
                      ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400'
                      : 'bg-slate-800 border-slate-700 text-slate-400'
                  }`}>
                    <ShieldCheck className="w-5 h-5" />
                  </div>
                  <div>
                    <h4 className="text-xs font-bold text-slate-100 flex items-center gap-2">
                      System Proxy Integration
                      <span className={`px-2 py-0.5 text-[10px] font-semibold rounded-full border ${
                        systemProxyEnabled
                          ? 'bg-emerald-500/20 text-emerald-300 border-emerald-500/30'
                          : 'bg-slate-800 text-slate-400 border-slate-700'
                      }`}>
                        {systemProxyEnabled ? 'Active' : 'Disabled'}
                      </span>
                    </h4>
                    <p className="text-[11px] text-slate-400 mt-0.5">
                      Automatically registers local HTTP and SOCKS5 endpoints in OS settings
                    </p>
                  </div>
                </div>

                <button
                  type="button"
                  onClick={handleToggleSystemProxy}
                  disabled={proxyLoading}
                  className={`px-4 py-2 rounded-xl text-xs font-semibold transition-all cursor-pointer flex items-center gap-1.5 ${
                    systemProxyEnabled
                      ? 'bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 border border-rose-500/40'
                      : 'bg-indigo-600 hover:bg-indigo-500 text-white shadow-indigo-600/20 shadow-md'
                  }`}
                >
                  {proxyLoading && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
                  <span>{systemProxyEnabled ? 'Disable Proxy' : 'Enable Proxy'}</span>
                </button>
              </div>

              {/* Endpoints Cards */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 flex items-center justify-between">
                  <div>
                    <span className="text-[10px] font-semibold uppercase tracking-wider text-slate-500">
                      HTTP Proxy Endpoint
                    </span>
                    <div className="text-xs font-mono font-bold text-indigo-300 mt-0.5">
                      {endpoints.httpHost}:{endpoints.httpPort}
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => copyText(`http://${endpoints.httpHost}:${endpoints.httpPort}`, 'http')}
                    className="p-1.5 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors cursor-pointer"
                    title="Copy HTTP Proxy URL"
                  >
                    {copiedKey === 'http' ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4" />}
                  </button>
                </div>

                <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800 flex items-center justify-between">
                  <div>
                    <span className="text-[10px] font-semibold uppercase tracking-wider text-slate-500">
                      SOCKS5 Proxy Endpoint
                    </span>
                    <div className="text-xs font-mono font-bold text-emerald-300 mt-0.5">
                      {endpoints.socks5Host}:{endpoints.socks5Port}
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => copyText(`socks5://${endpoints.socks5Host}:${endpoints.socks5Port}`, 'socks5')}
                    className="p-1.5 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors cursor-pointer"
                    title="Copy SOCKS5 Proxy URL"
                  >
                    {copiedKey === 'socks5' ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4" />}
                  </button>
                </div>
              </div>

              {/* Quick CLI Environment Setup */}
              <div className="p-4 rounded-xl bg-slate-950/70 border border-slate-800 space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2 text-xs font-semibold text-slate-300">
                    <Terminal className="w-3.5 h-3.5 text-indigo-400" />
                    <span>Terminal / CLI Environment Variables</span>
                  </div>

                  {/* Shell switch */}
                  <div className="flex items-center bg-slate-900 p-0.5 rounded-lg border border-slate-800 text-[10px]">
                    <button
                      type="button"
                      onClick={() => setCliShell('bash')}
                      className={`px-2 py-0.5 rounded cursor-pointer ${
                        cliShell === 'bash' ? 'bg-indigo-600 text-white' : 'text-slate-400 hover:text-slate-200'
                      }`}
                    >
                      Bash/Zsh
                    </button>
                    <button
                      type="button"
                      onClick={() => setCliShell('powershell')}
                      className={`px-2 py-0.5 rounded cursor-pointer ${
                        cliShell === 'powershell' ? 'bg-indigo-600 text-white' : 'text-slate-400 hover:text-slate-200'
                      }`}
                    >
                      PowerShell
                    </button>
                    <button
                      type="button"
                      onClick={() => setCliShell('cmd')}
                      className={`px-2 py-0.5 rounded cursor-pointer ${
                        cliShell === 'cmd' ? 'bg-indigo-600 text-white' : 'text-slate-400 hover:text-slate-200'
                      }`}
                    >
                      CMD
                    </button>
                  </div>
                </div>

                <div className="relative">
                  <pre className="p-3 bg-slate-900/90 rounded-lg border border-slate-800 font-mono text-[11px] text-slate-300 overflow-x-auto select-all">
                    {cliShell === 'bash' && (
                      `export http_proxy="http://${endpoints.httpHost}:${endpoints.httpPort}"\nexport https_proxy="http://${endpoints.httpHost}:${endpoints.httpPort}"\nexport all_proxy="socks5://${endpoints.socks5Host}:${endpoints.socks5Port}"`
                    )}
                    {cliShell === 'powershell' && (
                      `$env:http_proxy="http://${endpoints.httpHost}:${endpoints.httpPort}"\n$env:https_proxy="http://${endpoints.httpHost}:${endpoints.httpPort}"\n$env:all_proxy="socks5://${endpoints.socks5Host}:${endpoints.socks5Port}"`
                    )}
                    {cliShell === 'cmd' && (
                      `set http_proxy=http://${endpoints.httpHost}:${endpoints.httpPort}\nset https_proxy=http://${endpoints.httpHost}:${endpoints.httpPort}\nset all_proxy=socks5://${endpoints.socks5Host}:${endpoints.socks5Port}`
                    )}
                  </pre>
                  <button
                    type="button"
                    onClick={() => {
                      const snippet =
                        cliShell === 'bash'
                          ? `export http_proxy="http://${endpoints.httpHost}:${endpoints.httpPort}"\nexport https_proxy="http://${endpoints.httpHost}:${endpoints.httpPort}"\nexport all_proxy="socks5://${endpoints.socks5Host}:${endpoints.socks5Port}"`
                          : cliShell === 'powershell'
                          ? `$env:http_proxy="http://${endpoints.httpHost}:${endpoints.httpPort}"\n$env:https_proxy="http://${endpoints.httpHost}:${endpoints.httpPort}"\n$env:all_proxy="socks5://${endpoints.socks5Host}:${endpoints.socks5Port}"`
                          : `set http_proxy=http://${endpoints.httpHost}:${endpoints.httpPort}\nset https_proxy=http://${endpoints.httpHost}:${endpoints.httpPort}\nset all_proxy=socks5://${endpoints.socks5Host}:${endpoints.socks5Port}`;
                      copyText(snippet, 'cli');
                    }}
                    className="absolute top-2 right-2 p-1.5 rounded-md bg-slate-800/80 hover:bg-slate-700 text-slate-300 transition-colors cursor-pointer"
                    title="Copy commands"
                  >
                    {copiedKey === 'cli' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* TAB 3: BRIDGE */}
          {activeTab === 'bridge' && (
            <div className="animate-in fade-in duration-150">
              <BridgeView
                isConnected={isConnected}
                activeLabel={activeLabel}
                onConnect={onConnect}
              />
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
