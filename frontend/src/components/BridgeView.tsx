import React, { useState, useEffect } from 'react';
import {
  Plus,
  Pencil,
  Trash2,
  ToggleLeft,
  ToggleRight,
  AppWindow,
  Check,
  Layers,
  Copy,
  Terminal,
  X,
} from 'lucide-react';
import { api } from '../api/wails';
import type { BridgeRule, ProxyEndpointsDTO, InstalledApp } from '../types';
import { SelectAppModal } from './SelectAppModal';

interface BridgeViewProps {
  isConnected: boolean;
  activeLabel?: string;
  onConnect?: () => void;
}

export const BridgeView: React.FC<BridgeViewProps> = () => {
  const [rules, setRules] = useState<BridgeRule[]>([]);
  const [runningCounts, setRunningCounts] = useState<Record<string, number>>({});
  const [endpoints, setEndpoints] = useState<ProxyEndpointsDTO>({
    socks5Host: '127.0.0.1',
    socks5Port: 10808,
    httpHost: '127.0.0.1',
    httpPort: 10809,
    socks5Url: 'socks5://127.0.0.1:10808',
    httpUrl: 'http://127.0.0.1:10809',
  });
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [isSelectAppOpen, setIsSelectAppOpen] = useState(false);
  const [editingRule, setEditingRule] = useState<BridgeRule | null>(null);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  // Form state
  const [pattern, setPattern] = useState('');
  const [proxyType, setProxyType] = useState<'socks5' | 'http'>('socks5');
  const [customTarget, setCustomTarget] = useState('');
  const [description, setDescription] = useState('');

  // Load initial rules and proxy endpoints
  useEffect(() => {
    api.getBridgeRules().then(setRules);
    api.getProxyEndpoints().then(setEndpoints);
  }, []);

  // Poll running process counts every 3 seconds
  useEffect(() => {
    const checkProcesses = () => {
      api.checkRunningBridgeProcesses()
        .then((counts) => {
          if (counts) setRunningCounts(counts);
        })
        .catch(() => {});
    };

    checkProcesses();
    const interval = setInterval(checkProcesses, 3000);
    return () => clearInterval(interval);
  }, [rules]);

  const saveRules = async (updated: BridgeRule[]) => {
    setRules(updated);
    await api.saveBridgeRules(updated);
  };

  const handleToggleRule = async (id: string) => {
    const updated = rules.map((r) =>
      r.id === id ? { ...r, enabled: !r.enabled } : r
    );
    await saveRules(updated);
  };

  const handleDeleteRule = async (id: string) => {
    const updated = rules.filter((r) => r.id !== id);
    await saveRules(updated);
  };

  const handleOpenAddModal = () => {
    setEditingRule(null);
    setPattern('');
    setProxyType('socks5');
    setCustomTarget('');
    setDescription('');
    setIsModalOpen(true);
  };

  const handleOpenEditModal = (rule: BridgeRule) => {
    setEditingRule(rule);
    setPattern(rule.pattern);
    setProxyType(rule.proxyType);
    const isStandardTarget =
      rule.proxyTarget === `socks5://127.0.0.1:${endpoints.socks5Port}` ||
      rule.proxyTarget === `http://127.0.0.1:${endpoints.httpPort}` ||
      rule.proxyTarget === 'socks5://127.0.0.1:10808' ||
      rule.proxyTarget === 'http://127.0.0.1:10809';
    setCustomTarget(isStandardTarget ? '' : rule.proxyTarget);
    setDescription(rule.description || '');
    setIsModalOpen(true);
  };

  const handleSubmitRule = async (e: React.FormEvent) => {
    e.preventDefault();
    const cleanPattern = pattern.trim();
    if (!cleanPattern) return;

    let target = customTarget.trim();
    if (!target) {
      target =
        proxyType === 'http'
          ? `http://127.0.0.1:${endpoints.httpPort}`
          : `socks5://127.0.0.1:${endpoints.socks5Port}`;
    }

    if (editingRule) {
      const updated = rules.map((r) =>
        r.id === editingRule.id
          ? {
              ...r,
              pattern: cleanPattern,
              proxyTarget: target,
              proxyType,
              description: description.trim() || undefined,
            }
          : r
      );
      await saveRules(updated);
    } else {
      const newRule: BridgeRule = {
        id: 'rule-' + Date.now(),
        pattern: cleanPattern,
        proxyTarget: target,
        proxyType,
        enabled: true,
        description: description.trim() || undefined,
      };
      await saveRules([...rules, newRule]);
    }

    setIsModalOpen(false);
    setEditingRule(null);
  };

  const handleSelectAppForForm = (app: InstalledApp) => {
    const baseName = app.exePath.split(/[\\/]/).pop() || app.name;
    setPattern(baseName);
    if (!description) {
      setDescription(app.name);
    }
    setIsSelectAppOpen(false);
  };

  const handleBrowseManualForForm = async () => {
    try {
      const exe = await api.selectExecutableDialog();
      if (exe) {
        const baseName = exe.split(/[\\/]/).pop() || exe;
        setPattern(baseName);
        if (!description) {
          setDescription(exe);
        }
        setIsSelectAppOpen(false);
      }
    } catch (err: any) {
      console.error('File dialog error:', err);
    }
  };

  const copyToClipboard = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  return (
    <div className="space-y-6 pb-8 animate-in fade-in duration-200">
      {/* Top Banner / Controls */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-5 rounded-2xl bg-slate-900/60 border border-slate-800 backdrop-blur-xs">
        <div>
          <div className="flex items-center gap-2">
            <h2 className="text-base font-semibold text-slate-100 flex items-center gap-2">
              <Layers className="w-4 h-4 text-indigo-400" />
              Bridge Mode Rules
            </h2>
            <span className="px-2 py-0.5 rounded-full text-[10px] font-medium bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
              Regex & Wildcard
            </span>
          </div>
          <p className="text-xs text-slate-400 mt-0.5">
            Executable name, supports wildcards (*, ?) and regex.
          </p>
        </div>

        {/* Action Button */}
        <div className="flex items-center gap-2">
          <button
            onClick={handleOpenAddModal}
            className="px-3.5 py-1.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium flex items-center gap-1.5 transition-colors shadow-sm cursor-pointer"
          >
            <Plus className="w-3.5 h-3.5" />
            Add Rule
          </button>
        </div>
      </div>

      {/* Rules Table */}
      <div className="border border-slate-800 rounded-2xl bg-slate-900/40 overflow-hidden shadow-xs">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-slate-800 bg-slate-900/80 text-[11px] font-semibold text-slate-400 uppercase tracking-wider">
                <th className="py-3 px-4">Process / Pattern</th>
                <th className="py-3 px-4">Proxy Target</th>
                <th className="py-3 px-4">Live Status</th>
                <th className="py-3 px-4 text-center">Enabled</th>
                <th className="py-3 px-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 text-xs">
              {rules.length === 0 ? (
                <tr>
                  <td colSpan={5} className="py-10 text-center text-slate-500">
                    <p className="text-sm font-medium text-slate-400">No Bridge rules configured</p>
                    <p className="text-xs text-slate-500 mt-1">
                      Click "+ Add Rule" to route matching processes through proxy.
                    </p>
                  </td>
                </tr>
              ) : (
                rules.map((rule) => {
                  const runningCount = runningCounts[rule.id] || 0;
                  const isRunning = runningCount > 0;

                  return (
                    <tr
                      key={rule.id}
                      className="hover:bg-slate-800/30 transition-colors group"
                    >
                      {/* Pattern */}
                      <td className="py-3 px-4">
                        <div className="flex flex-col">
                          <span className="font-mono text-slate-200 font-medium">
                            {rule.pattern}
                          </span>
                          {rule.description && (
                            <span className="text-[10px] text-slate-500 truncate max-w-xs" title={rule.description}>
                              {rule.description}
                            </span>
                          )}
                        </div>
                      </td>

                      {/* Proxy Target */}
                      <td className="py-3 px-4">
                        <div className="flex items-center gap-1.5">
                          <span
                            className={`px-1.5 py-0.5 rounded-sm text-[10px] font-mono font-semibold uppercase ${
                              rule.proxyType === 'http'
                                ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                                : 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                            }`}
                          >
                            {rule.proxyType}
                          </span>
                          <span className="font-mono text-[11px] text-slate-400">
                            {rule.proxyTarget}
                          </span>
                        </div>
                      </td>

                      {/* Status */}
                      <td className="py-3 px-4">
                        {isRunning ? (
                          <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
                            Running ({runningCount})
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-medium bg-slate-800 text-slate-400 border border-slate-700/50">
                            <span className="w-1.5 h-1.5 rounded-full bg-slate-500" />
                            Inactive
                          </span>
                        )}
                      </td>

                      {/* Enabled Toggle */}
                      <td className="py-3 px-4 text-center">
                        <button
                          onClick={() => handleToggleRule(rule.id)}
                          className="text-slate-400 hover:text-slate-200 transition-colors cursor-pointer"
                        >
                          {rule.enabled ? (
                            <ToggleRight className="w-6 h-6 text-indigo-500" />
                          ) : (
                            <ToggleLeft className="w-6 h-6 text-slate-600" />
                          )}
                        </button>
                      </td>

                      {/* Actions */}
                      <td className="py-3 px-4 text-right">
                        <div className="flex items-center justify-end gap-1.5">
                          <button
                            onClick={() => handleOpenEditModal(rule)}
                            title="Edit Rule"
                            className="p-1.5 rounded-lg text-slate-400 hover:text-indigo-400 hover:bg-indigo-500/10 transition-colors cursor-pointer"
                          >
                            <Pencil className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => handleDeleteRule(rule.id)}
                            title="Delete Rule"
                            className="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Manual Endpoints Card */}
      <div className="p-5 rounded-2xl bg-slate-900/40 border border-slate-800">
        <h3 className="text-xs font-semibold text-slate-300 flex items-center gap-2 mb-3">
          <Terminal className="w-4 h-4 text-indigo-400" />
          Active Proxy Endpoints
        </h3>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
          <div className="p-3 rounded-xl bg-slate-950 border border-slate-800/80 flex items-center justify-between">
            <div>
              <div className="text-[10px] text-slate-500 font-semibold uppercase">SOCKS5 Endpoint</div>
              <div className="font-mono text-slate-200 mt-0.5">{endpoints.socks5Url}</div>
            </div>
            <button
              onClick={() => copyToClipboard(endpoints.socks5Url, 'socks')}
              className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors cursor-pointer"
            >
              {copiedKey === 'socks' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            </button>
          </div>

          <div className="p-3 rounded-xl bg-slate-950 border border-slate-800/80 flex items-center justify-between">
            <div>
              <div className="text-[10px] text-slate-500 font-semibold uppercase">HTTP Endpoint</div>
              <div className="font-mono text-slate-200 mt-0.5">{endpoints.httpUrl}</div>
            </div>
            <button
              onClick={() => copyToClipboard(endpoints.httpUrl, 'http')}
              className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors cursor-pointer"
            >
              {copiedKey === 'http' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            </button>
          </div>
        </div>
      </div>

      {/* Add / Edit Rule Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-150">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md shadow-2xl overflow-hidden flex flex-col">
            <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-900/50">
              <h3 className="text-base font-semibold text-slate-100">
                {editingRule ? 'Edit Bridge Rule' : 'Add Bridge Rule'}
              </h3>
              <button
                onClick={() => setIsModalOpen(false)}
                className="p-1.5 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleSubmitRule} className="p-6 space-y-4">
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-slate-300">
                  Process Pattern (Wildcard / Regex)
                </label>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={pattern}
                    onChange={(e) => setPattern(e.target.value)}
                    placeholder="e.g. Discovery*.exe, curl, or ^Discord.*"
                    className="flex-1 px-3.5 py-2 rounded-xl bg-slate-950 border border-slate-800 text-slate-100 text-xs font-mono placeholder:text-slate-600 focus:outline-hidden focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-all"
                    autoFocus
                  />
                  <button
                    type="button"
                    onClick={() => setIsSelectAppOpen(true)}
                    className="px-3 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium flex items-center gap-1.5 transition-colors border border-slate-700/50 shrink-0 cursor-pointer"
                    title="Browse installed apps or files"
                  >
                    <AppWindow className="w-3.5 h-3.5 text-indigo-400" />
                    Browse
                  </button>
                </div>
                <p className="text-[11px] text-slate-500">
                  Executable name, supports wildcards (*, ?) or regex.
                </p>
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-medium text-slate-300">Proxy Protocol</label>
                <div className="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => setProxyType('socks5')}
                    className={`px-3 py-2 rounded-xl text-xs font-medium border transition-colors cursor-pointer ${
                      proxyType === 'socks5'
                        ? 'bg-indigo-600 text-white border-indigo-500'
                        : 'bg-slate-950 text-slate-400 border-slate-800 hover:bg-slate-800 hover:text-slate-200'
                    }`}
                  >
                    SOCKS5 (127.0.0.1:{endpoints.socks5Port})
                  </button>
                  <button
                    type="button"
                    onClick={() => setProxyType('http')}
                    className={`px-3 py-2 rounded-xl text-xs font-medium border transition-colors cursor-pointer ${
                      proxyType === 'http'
                        ? 'bg-indigo-600 text-white border-indigo-500'
                        : 'bg-slate-950 text-slate-400 border-slate-800 hover:bg-slate-800 hover:text-slate-200'
                    }`}
                  >
                    HTTP (127.0.0.1:{endpoints.httpPort})
                  </button>
                </div>
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-medium text-slate-300 flex items-center justify-between">
                  <span>Custom Proxy Target</span>
                  <span className="text-[10px] text-slate-500">Optional</span>
                </label>
                <input
                  type="text"
                  value={customTarget}
                  onChange={(e) => setCustomTarget(e.target.value)}
                  placeholder={
                    proxyType === 'http'
                      ? `http://127.0.0.1:${endpoints.httpPort}`
                      : `socks5://127.0.0.1:${endpoints.socks5Port}`
                  }
                  className="w-full px-3.5 py-2 rounded-xl bg-slate-950 border border-slate-800 text-slate-100 text-xs font-mono placeholder:text-slate-600 focus:outline-hidden focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-all"
                />
              </div>

              <div className="space-y-1.5">
                <label className="text-xs font-medium text-slate-300 flex items-center justify-between">
                  <span>Description / Executable Path</span>
                  <span className="text-[10px] text-slate-500">Optional</span>
                </label>
                <input
                  type="text"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="e.g. C:\Games\Discovery\Discovery.exe"
                  className="w-full px-3.5 py-2 rounded-xl bg-slate-950 border border-slate-800 text-slate-100 text-xs placeholder:text-slate-600 focus:outline-hidden focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-all"
                />
              </div>

              <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-800/80">
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  className="px-4 py-2 rounded-xl text-xs font-medium text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={!pattern.trim()}
                  className="px-5 py-2 rounded-xl text-xs font-medium text-white bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 disabled:pointer-events-none transition-colors shadow-sm cursor-pointer"
                >
                  {editingRule ? 'Save Changes' : 'Add Rule'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Select Installed App Modal */}
      <SelectAppModal
        isOpen={isSelectAppOpen}
        onClose={() => setIsSelectAppOpen(false)}
        onSelectApp={handleSelectAppForForm}
        onBrowseManual={handleBrowseManualForForm}
      />
    </div>
  );
};
