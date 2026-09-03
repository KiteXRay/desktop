import React, { useState, useEffect } from 'react';
import {
  Copy,
  Check,
  Rocket,
  ToggleLeft,
  ToggleRight,
  Terminal,
  Loader2,
  Trash2,
  Play
} from 'lucide-react';
import { api } from '../api/wails';
import type { ProxyEndpointsDTO, InstalledApp } from '../types';
import { SelectAppModal } from './SelectAppModal';

interface TunneledApp {
  id: string;
  name: string;
  path: string;
  icon?: string;
  desc?: string;
}

interface PerAppViewProps {
  isConnected: boolean;
  activeLabel?: string;
  onConnect?: () => void;
}

export const PerAppView: React.FC<PerAppViewProps> = ({
  isConnected,
  activeLabel,
  onConnect
}) => {
  const [endpoints, setEndpoints] = useState<ProxyEndpointsDTO>({
    socks5Host: '127.0.0.1',
    socks5Port: 10808,
    httpHost: '127.0.0.1',
    httpPort: 10809,
    socks5Url: 'socks5://127.0.0.1:10808',
    httpUrl: 'http://127.0.0.1:10809'
  });
  const [systemProxy, setSystemProxy] = useState<boolean>(false);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [launchingApp, setLaunchingApp] = useState<string | null>(null);
  const [launchMessage, setLaunchMessage] = useState<string | null>(null);
  const [isSelectAppModalOpen, setIsSelectAppModalOpen] = useState(false);
  const [customApps, setCustomApps] = useState<TunneledApp[]>(() => {
    try {
      const saved = localStorage.getItem('kite_tunneled_apps');
      return saved ? JSON.parse(saved) : [];
    } catch {
      return [];
    }
  });

  useEffect(() => {
    api.getProxyEndpoints().then(setEndpoints);
    api.getSystemProxyStatus().then(setSystemProxy);

    const unsub = api.onProxyStatusChanged((active) => {
      setSystemProxy(active);
    });
    return () => {
      unsub();
    };
  }, [isConnected]);

  const copyToClipboard = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const handleToggleSystemProxy = async () => {
    const nextState = !systemProxy;
    setSystemProxy(nextState);
    await api.setSystemProxy(nextState);
  };

  const handleLaunchApp = async (appName: string, path: string = '') => {
    setLaunchingApp(appName);
    setLaunchMessage(null);
    try {
      if (!isConnected && onConnect) {
        setLaunchMessage('Starting proxy...');
        await onConnect();
      }

      if (path) {
        await api.launchAppWithProxy('', path);
      } else {
        await api.launchAppWithProxy(appName);
      }
      setLaunchMessage(`Launched ${appName} with Kite proxy!`);
      setTimeout(() => setLaunchMessage(null), 3500);
    } catch (err: any) {
      setLaunchMessage(`Launch error: ${err?.message || err}`);
    } finally {
      setLaunchingApp(null);
    }
  };

  const handleBrowseExecutable = async () => {
    try {
      const selected = await api.selectExecutableDialog();
      if (!selected) return;

      const fileName = selected.split(/[/\\]/).pop() || selected;
      const baseName = fileName.replace(/\.[^/.]+$/, '');

      setCustomApps(prev => {
        if (prev.some(a => a.path.toLowerCase() === selected.toLowerCase())) {
          return prev;
        }
        const updated = [{ id: 'app-' + Date.now(), name: baseName, path: selected }, ...prev];
        localStorage.setItem('kite_tunneled_apps', JSON.stringify(updated));
        return updated;
      });

      await handleLaunchApp(baseName, selected);
    } catch (err: any) {
      setLaunchMessage(`Error selecting file: ${err?.message || err}`);
    }
  };

  const handleAppSelectedFromModal = async (app: InstalledApp) => {
    setIsSelectAppModalOpen(false);
    setCustomApps(prev => {
      if (prev.some(a => a.path.toLowerCase() === app.exePath.toLowerCase())) {
        return prev;
      }
      const updated = [{ id: 'app-' + Date.now(), name: app.name, path: app.exePath, icon: app.icon }, ...prev];
      localStorage.setItem('kite_tunneled_apps', JSON.stringify(updated));
      return updated;
    });

    await handleLaunchApp(app.name, app.exePath);
  };

  const handleRemoveCustomApp = (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    setCustomApps(prev => {
      const updated = prev.filter(a => a.id !== id);
      localStorage.setItem('kite_tunneled_apps', JSON.stringify(updated));
      return updated;
    });
  };

  return (
    <div className="max-w-4xl mx-auto py-6 px-4 flex flex-col gap-6 animate-in fade-in duration-200">
      <div className="bg-gradient-to-r from-indigo-950/70 via-slate-900/80 to-purple-950/70 rounded-2xl p-6 border border-indigo-500/30 shadow-xl flex flex-col md:flex-row items-start md:items-center justify-between gap-5">
        <div className="flex items-center gap-4">
          <div className="w-14 h-14 rounded-2xl bg-indigo-600/20 border border-indigo-500/40 flex items-center justify-center text-indigo-400 shrink-0 shadow-lg shadow-indigo-600/10">
            <Rocket className="w-7 h-7" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h2 className="text-base font-bold text-slate-100">
                App Tunnel
              </h2>
              <span className={`text-[10px] uppercase font-mono px-2 py-0.5 rounded-full border ${
                isConnected ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30' : 'bg-slate-800 text-slate-400 border-slate-700'
              }`}>
                {isConnected ? `Active (${activeLabel || 'Connected'})` : 'Idle'}
              </span>
            </div>
            <p className="text-xs text-slate-300 mt-1 max-w-lg leading-relaxed">
              Select any executable (<code className="text-indigo-300 font-mono">.exe</code> on Windows or binary on Linux). Kite will connect, start the proxy, and route only that application through the VPN.
            </p>
          </div>
        </div>

        <button
          onClick={() => setIsSelectAppModalOpen(true)}
          disabled={launchingApp !== null}
          className="flex items-center gap-2 px-5 py-3 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs shadow-lg shadow-indigo-600/30 active:scale-95 transition-all shrink-0 cursor-pointer disabled:opacity-50"
        >
          {launchingApp ? (
            <>
              <Loader2 className="w-4 h-4 animate-spin text-white" />
              <span>Routing Application...</span>
            </>
          ) : (
            <>
              <Rocket className="w-4 h-4" />
              <span>Select App</span>
            </>
          )}
        </button>
      </div>

      {launchMessage && (
        <div className="flex items-center gap-2 p-3 rounded-xl bg-indigo-500/10 border border-indigo-500/20 text-indigo-300 text-xs font-medium animate-in fade-in">
          <Rocket className="w-4 h-4 text-indigo-400 shrink-0" />
          <span>{launchMessage}</span>
        </div>
      )}

      {customApps.length > 0 && (
        <div className="bg-slate-900/50 rounded-2xl p-5 border border-slate-800 flex flex-col gap-3.5">
          <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-400 flex items-center justify-between">
            <span>Tunneled Applications ({customApps.length})</span>
            <span className="font-normal text-[11px] text-slate-500">1-click launch with proxy</span>
          </h3>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
            {customApps.map((app) => (
              <div
                key={app.id}
                className="flex items-center justify-between p-3 rounded-xl border border-slate-800 bg-slate-900/70 hover:border-slate-700 transition-all group"
              >
                <div className="flex items-center gap-2.5 min-w-0 mr-2">
                  <div className="w-8 h-8 rounded-lg bg-slate-800 flex items-center justify-center text-sm shrink-0 overflow-hidden border border-slate-700/50">
                    {app.icon ? (
                      <img src={app.icon} alt={app.name} className="w-full h-full object-contain p-0.5" />
                    ) : (
                      <span className="font-bold text-xs uppercase text-slate-300">{app.name.charAt(0)}</span>
                    )}
                  </div>
                  <div className="min-w-0">
                    <p className="text-xs font-semibold text-slate-200 truncate">{app.name}</p>
                    <p className="text-[10px] font-mono text-slate-500 truncate" title={app.path}>{app.path}</p>
                  </div>
                </div>

                <div className="flex items-center gap-1.5 shrink-0">
                  <button
                    onClick={() => handleLaunchApp(app.name, app.path)}
                    disabled={launchingApp !== null}
                    className="flex items-center gap-1 px-2.5 py-1 rounded-lg bg-indigo-600/20 hover:bg-indigo-600 text-indigo-300 hover:text-white text-xs font-medium transition-colors cursor-pointer"
                    title="Launch application routed through proxy"
                  >
                    <Play className="w-3 h-3" />
                    <span>Launch</span>
                  </button>
                  <button
                    onClick={(e) => handleRemoveCustomApp(app.id, e)}
                    className="p-1.5 rounded-lg text-slate-500 hover:text-rose-400 hover:bg-slate-800 transition-colors cursor-pointer"
                    title="Remove from list"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div className="bg-slate-900/50 rounded-2xl p-4 border border-slate-800 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-semibold uppercase tracking-wider text-indigo-400 flex items-center gap-1.5">
                <span className="w-2 h-2 rounded-full bg-indigo-500" />
                SOCKS5 Endpoint
              </span>
              <button
                onClick={() => copyToClipboard(`${endpoints.socks5Host}:${endpoints.socks5Port}`, 'socks')}
                className="hover:text-indigo-400 transition-colors p-1 text-slate-400 cursor-pointer"
                title="Copy host:port"
              >
                {copiedKey === 'socks' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
              </button>
            </div>
            <div className="flex items-baseline justify-between font-mono text-xs bg-slate-950/60 p-2.5 rounded-xl border border-slate-800/80">
              <span className="text-slate-300">{endpoints.socks5Host}</span>
              <span className="text-indigo-400 font-bold">:{endpoints.socks5Port}</span>
            </div>
          </div>
          <div className="mt-3 text-[11px] text-slate-400 flex items-center gap-1.5">
            <Terminal className="w-3.5 h-3.5 text-slate-500 shrink-0" />
            <span className="font-mono text-[10px] text-slate-300">socks5://{endpoints.socks5Host}:{endpoints.socks5Port}</span>
          </div>
        </div>

        <div className="bg-slate-900/50 rounded-2xl p-4 border border-slate-800 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-semibold uppercase tracking-wider text-emerald-400 flex items-center gap-1.5">
                <span className="w-2 h-2 rounded-full bg-emerald-500" />
                HTTP / HTTPS Endpoint
              </span>
              <button
                onClick={() => copyToClipboard(`${endpoints.httpHost}:${endpoints.httpPort}`, 'http')}
                className="hover:text-indigo-400 transition-colors p-1 text-slate-400 cursor-pointer"
                title="Copy host:port"
              >
                {copiedKey === 'http' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
              </button>
            </div>
            <div className="flex items-baseline justify-between font-mono text-xs bg-slate-950/60 p-2.5 rounded-xl border border-slate-800/80">
              <span className="text-slate-300">{endpoints.httpHost}</span>
              <span className="text-emerald-400 font-bold">:{endpoints.httpPort}</span>
            </div>
          </div>
          <div className="mt-3 text-[11px] text-slate-400 flex items-center justify-between">
            <div className="flex items-center gap-1.5">
              <Terminal className="w-3.5 h-3.5 text-slate-500 shrink-0" />
              <span className="font-mono text-[10px] text-slate-300">http://{endpoints.httpHost}:{endpoints.httpPort}</span>
            </div>
            <button
              onClick={handleToggleSystemProxy}
              className="flex items-center gap-1 text-[10px] text-slate-400 hover:text-slate-200 cursor-pointer"
              title="Toggle OS System Proxy"
            >
              <span>System Proxy:</span>
              {systemProxy ? (
                <ToggleRight className="w-4 h-4 text-emerald-400" />
              ) : (
                <ToggleLeft className="w-4 h-4 text-slate-600" />
              )}
            </button>
          </div>
        </div>
      </div>

      <SelectAppModal
        isOpen={isSelectAppModalOpen}
        onClose={() => setIsSelectAppModalOpen(false)}
        onSelectApp={handleAppSelectedFromModal}
        onBrowseManual={() => {
          setIsSelectAppModalOpen(false);
          handleBrowseExecutable();
        }}
        isLaunching={launchingApp !== null}
      />
    </div>
  );
};
