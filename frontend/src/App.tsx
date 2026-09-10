import { useState, useEffect, useCallback, useMemo } from 'react';
import {
  Plus,
  Power,
  Activity,
  Info,
  AlertCircle,
  CheckCircle2,
  Server,
  RotateCcw,
  Loader2,
  LogOut,
  ArrowUp,
  ArrowDown,
  Globe,
  Settings,
  Network,
  Layers,
  Copy,
  ChevronDown,
  RotateCw,
  Trash2,
  HardDrive,
  Rss,
  LayoutList,
  List,
  MoreVertical,
  X,
} from 'lucide-react';
import { api } from './api/wails';
import type { ConnectionDTO, TunnelMode, ReleaseInfo, UpdateProgress, NetworkPrivilegesDTO, Subscription, AppInfoDTO } from './types';
import { ProfileCard } from './components/ProfileCard';
import { NetworkChart } from './components/NetworkChart';
import { AddEditModal } from './components/AddEditModal';
import { UpdateModal } from './components/UpdateModal';
import { PrivilegeModal } from './components/PrivilegeModal';
import { AboutView } from './components/AboutView';
import { ModeSettingsModal } from './components/ModeSettingsModal';
import { formatBytes } from './utils/formatters';
import { useConnections } from './hooks/useConnections';

export function App() {
  const [toast, setToast] = useState<{ message: string; type: 'success' | 'error' | 'info' } | null>(null);

  const showToast = useCallback((message: string, type: 'success' | 'error' | 'info' = 'info') => {
    setToast({ message, type });
    setTimeout(() => setToast(null), 3500);
  }, []);

  const {
    connections,
    setSelectedId,
    activeStats,
    setActiveStats,
    connectingId,
    disconnectingId,
    activeConnection,
    selectedConnection,
    loadConnections,
    handleConnect,
    handleDisconnect,
    handleDelete,
    handleResetTraffic,
    handleReorder,
  } = useConnections({ showToast });

  const [appInfo, setAppInfo] = useState<AppInfoDTO | null>(null);
  const [isCompactMode, setIsCompactMode] = useState<boolean>(() => {
    try {
      return localStorage.getItem('kite_compact_mode') === 'true';
    } catch {
      return false;
    }
  });
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [editItem, setEditItem] = useState<ConnectionDTO | null>(null);
  const [currentTab, setCurrentTab] = useState<'connections' | 'about'>('connections');
  const [tunnelMode, setTunnelMode] = useState<TunnelMode>('tunnel');
  const [isModeSettingsOpen, setIsModeSettingsOpen] = useState(false);
  const [settingsTab, setSettingsTab] = useState<'tunnel' | 'proxy' | 'bridge' | 'general'>('tunnel');
  const [isClearingTun, setIsClearingTun] = useState(false);
  const [draggedIndex, setDraggedIndex] = useState<number | null>(null);
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null);
  const [updateInfo, setUpdateInfo] = useState<ReleaseInfo | null>(null);
  const [isUpdateModalOpen, setIsUpdateModalOpen] = useState(false);
  const [updateProgress, setUpdateProgress] = useState<UpdateProgress | null>(null);
  const [lastActiveId, setLastActiveId] = useState<string | null>(() => {
    try {
      return localStorage.getItem('kite_last_active_id');
    } catch {
      return null;
    }
  });

  const [privilegeInfo, setPrivilegeInfo] = useState<NetworkPrivilegesDTO | null>(null);
  const [isPrivilegeModalOpen, setIsPrivilegeModalOpen] = useState(false);
  const [pings, setPings] = useState<Record<string, number>>({});
  const [pingingIds, setPingingIds] = useState<Record<string, boolean>>({});
  const [isPingingAll, setIsPingingAll] = useState(false);
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([]);
  const [updatingSubIds, setUpdatingSubIds] = useState<Record<string, boolean>>({});
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});

  const loadSubscriptions = useCallback(async () => {
    try {
      const subs = await api.getSubscriptions();
      setSubscriptions(subs || []);
    } catch (err) {
      console.debug('Failed to load subscriptions:', err);
    }
  }, []);

  useEffect(() => {
    loadSubscriptions();
  }, [loadSubscriptions, connections]);

  useEffect(() => {
    const handleVisibilityChange = () => {
      const isVisible = document.visibilityState === 'visible';
      api.notifyWindowVisibility(isVisible);
    };
    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => document.removeEventListener('visibilitychange', handleVisibilityChange);
  }, []);

  useEffect(() => {
    api.getAppInfo().then(setAppInfo).catch(() => {});

    api.getCompactMode().then((val) => {
      setIsCompactMode(val);
      try { localStorage.setItem('kite_compact_mode', String(val)); } catch {}
    }).catch(() => {});

    const unsubCompact = api.onCompactModeChanged((val) => {
      setIsCompactMode(val);
      try { localStorage.setItem('kite_compact_mode', String(val)); } catch {}
    });

    return () => {
      unsubCompact();
    };
  }, []);

  const handleToggleCompactMode = async () => {
    const next = !isCompactMode;
    setIsCompactMode(next);
    try { localStorage.setItem('kite_compact_mode', String(next)); } catch {}
    try {
      await api.setCompactMode(next);
    } catch (err) {
      console.error('Failed to save compact mode:', err);
    }
  };

  const toggleGroupCollapse = (groupId: string) => {
    setCollapsedGroups(prev => ({ ...prev, [groupId]: !prev[groupId] }));
  };

  const handleUpdateSubscription = async (subId: string) => {
    setUpdatingSubIds(prev => ({ ...prev, [subId]: true }));
    try {
      await api.updateSubscription(subId);
      await loadConnections();
      await loadSubscriptions();
      showToast('Subscription updated successfully', 'success');
    } catch (err: any) {
      showToast(err?.message || 'Failed to update subscription', 'error');
    } finally {
      setUpdatingSubIds(prev => ({ ...prev, [subId]: false }));
    }
  };

  const handleDeleteSubscription = async (subId: string, subLabel: string) => {
    if (!window.confirm(`Delete subscription "${subLabel}" and all its profiles?`)) {
      return;
    }
    try {
      await api.deleteSubscription(subId);
      await loadConnections();
      await loadSubscriptions();
      showToast(`Subscription "${subLabel}" deleted`, 'info');
    } catch (err: any) {
      showToast(err?.message || 'Failed to delete subscription', 'error');
    }
  };

  interface ProfileGroup {
    id: string;
    title: string;
    type: 'local' | 'subscription';
    subscription?: Subscription;
    items: Array<{
      item: ConnectionDTO;
      globalIndex: number;
    }>;
  }

  function getSubscriptionGroupName(sub: Subscription): string {
    if (sub.subId) {
      return `Subscription-${sub.subId}`;
    }
    if (sub.label && sub.label.startsWith('Subscription-')) {
      return sub.label;
    }
    const match = sub.url?.match(/\/(?:sub|clash|subscription|subscribe)\/([a-zA-Z0-9_-]+)/i);
    if (match && match[1]) {
      return `Subscription-${match[1]}`;
    }
    try {
      const urlObj = new URL(sub.url);
      const token = urlObj.searchParams.get('token') || urlObj.searchParams.get('subid') || urlObj.searchParams.get('id');
      if (token) {
        return `Subscription-${token}`;
      }
    } catch {}
    return sub.label || `Subscription-${sub.id.replace(/^sub-/, '')}`;
  }

  const groups = useMemo<ProfileGroup[]>(() => {
    const result: ProfileGroup[] = [];

    // 1. Local group (items without subscriptionId)
    const localItems: Array<{ item: ConnectionDTO; globalIndex: number }> = [];
    connections.forEach((item, idx) => {
      if (!item.subscriptionId) {
        localItems.push({ item, globalIndex: idx });
      }
    });

    if (localItems.length > 0 || subscriptions.length === 0) {
      result.push({
        id: 'local',
        title: 'Local',
        type: 'local',
        items: localItems,
      });
    }

    // 2. Subscription groups
    subscriptions.forEach((sub) => {
      const subItems: Array<{ item: ConnectionDTO; globalIndex: number }> = [];
      connections.forEach((item, idx) => {
        if (item.subscriptionId === sub.id) {
          subItems.push({ item, globalIndex: idx });
        }
      });

      result.push({
        id: sub.id,
        title: getSubscriptionGroupName(sub),
        type: 'subscription',
        subscription: sub,
        items: subItems,
      });
    });

    // 3. Catch-all for any orphaned subscription items
    const knownSubIds = new Set(subscriptions.map(s => s.id));
    const orphanItems: Array<{ item: ConnectionDTO; globalIndex: number }> = [];
    connections.forEach((item, idx) => {
      if (item.subscriptionId && !knownSubIds.has(item.subscriptionId)) {
        orphanItems.push({ item, globalIndex: idx });
      }
    });
    if (orphanItems.length > 0) {
      result.push({
        id: 'other-subscriptions',
        title: 'Other Subscriptions',
        type: 'subscription',
        items: orphanItems,
      });
    }

    return result;
  }, [connections, subscriptions]);

  useEffect(() => {
    // Check network privileges on startup
    api.checkNetworkPrivileges().then(info => {
      if (info && !info.hasPrivileges) {
        setPrivilegeInfo(info);
        setIsPrivilegeModalOpen(true);
      }
    }).catch(() => {});

    // Listen for runtime privilege errors
    const unsubPrivs = api.onNetworkPrivilegesRequired(data => {
      setPrivilegeInfo(prev => ({
        hasPrivileges: false,
        os: prev?.os || 'linux',
        exePath: prev?.exePath || '',
        command: data.command,
        error: data.error,
      }));
      setIsPrivilegeModalOpen(true);
    });

    const unsubProgress = api.onUpdateProgress(prog => {
      setUpdateProgress(prog);
    });

    const unsubPing = api.onPingResult(res => {
      setPings(prev => ({ ...prev, [res.id]: res.pingMs }));
      setPingingIds(prev => ({ ...prev, [res.id]: false }));
    });

    const unsubPingStart = api.onPingStart(id => {
      setPingingIds(prev => ({ ...prev, [id]: true }));
    });

    const unsubStatus = api.onConnectionStatus(event => {
      if (event.status === 'connected' && event.id) {
        setPingingIds(prev => ({ ...prev, [event.id]: true }));
      }
    });

    const timer = setTimeout(async () => {
      try {
        const info = await api.checkForUpdate();
        if (info && info.available) {
          setUpdateInfo(info);
          const skipped = localStorage.getItem('kite_skipped_update_version');
          const snoozedUntil = Number(localStorage.getItem('kite_snoozed_update_until') || 0);
          if (skipped === info.latestVersion) {
            return;
          }
          if (Date.now() < snoozedUntil) {
            return;
          }
          setIsUpdateModalOpen(true);
        }
      } catch (err) {
        console.debug('Background update check:', err);
      }
    }, 2000);

    api.getTunnelMode().then((mode) => {
      if (mode === 'proxy' || mode === 'bridge' || mode === 'tunnel') {
        setTunnelMode(mode);
      } else if (mode === 'per_app') {
        setTunnelMode('bridge');
      } else {
        setTunnelMode('tunnel');
      }
    }).catch(() => {});

    const unsubMode = api.onModeChanged((mode) => {
      if (mode === 'proxy' || mode === 'bridge' || mode === 'tunnel') {
        setTunnelMode(mode as TunnelMode);
      } else if (mode === 'per_app') {
        setTunnelMode('bridge');
      } else {
        setTunnelMode('tunnel');
      }
    });

    return () => {
      unsubPrivs();
      unsubProgress();
      unsubPing();
      unsubPingStart();
      unsubStatus();
      unsubMode();
      clearTimeout(timer);
    };
  }, []);

  const handleCheckPrivilegesAgain = async (): Promise<boolean> => {
    const info = await api.checkNetworkPrivileges();
    if (info && info.hasPrivileges) {
      setPrivilegeInfo(info);
      showToast('Network privileges verified successfully!', 'success');
      return true;
    }
    return false;
  };

  const handleGrantWithPkexec = async (): Promise<boolean> => {
    const ok = await api.grantNetworkPrivileges();
    if (ok) {
      const info = await api.checkNetworkPrivileges();
      setPrivilegeInfo(info);
      showToast('Network privileges granted successfully!', 'success');
      return true;
    }
    return false;
  };

  const handleInstallUpdate = async (assetUrl: string, releaseUrl: string) => {
    try {
      await api.installUpdate(assetUrl, releaseUrl);
    } catch (err: any) {
      showToast(`Update error: ${err?.message || err}`, 'error');
    }
  };

  const handleCancelUpdate = async () => {
    try {
      await api.cancelUpdate();
      setUpdateProgress(null);
      showToast('Update download cancelled', 'info');
    } catch (err: any) {
      console.error('Failed to cancel update:', err);
    }
  };

  const handleSnoozeUpdate = () => {
    const until = Date.now() + 24 * 60 * 60 * 1000;
    localStorage.setItem('kite_snoozed_update_until', String(until));
    setIsUpdateModalOpen(false);
    showToast('Update reminder snoozed for 24 hours', 'info');
  };

  const handleSkipVersion = (version: string) => {
    localStorage.setItem('kite_skipped_update_version', version);
    setIsUpdateModalOpen(false);
    showToast(`Skipped version ${version}`, 'info');
  };

  useEffect(() => {
    if (activeConnection?.id) {
      setLastActiveId(activeConnection.id);
      localStorage.setItem('kite_last_active_id', activeConnection.id);
    }
  }, [activeConnection?.id]);

  const targetConnection = selectedConnection
    || connections.find(c => c.id === lastActiveId)
    || connections[0]
    || null;

  const handleModeChange = async (newMode: TunnelMode) => {
    if (tunnelMode === newMode) return;
    setTunnelMode(newMode);
    try {
      await api.setTunnelMode(newMode);
      const labels: Record<string, string> = {
        tunnel: 'System Tunnel',
        system: 'System Tunnel',
        proxy: 'System Proxy',
        bridge: 'Bridge Rules',
        per_app: 'Bridge Rules',
      };
      showToast(
        `Switched to ${labels[newMode] || newMode}`,
        'info'
      );
    } catch (err: any) {
      showToast(`Failed to switch mode: ${err?.message || err}`, 'error');
    }
  };

  const handleOpenSettings = (tab?: 'tunnel' | 'proxy' | 'bridge' | 'general') => {
    const target = tab || (tunnelMode === 'bridge' || tunnelMode === 'per_app' ? 'bridge' : tunnelMode === 'proxy' ? 'proxy' : 'tunnel');
    setSettingsTab(target);
    setIsModeSettingsOpen(true);
  };

  const handleSmartConnect = () => {
    if (activeConnection) {
      handleDisconnect();
    } else if (targetConnection) {
      handleConnect(targetConnection.id);
    } else {
      setIsAddModalOpen(true);
    }
  };

  const handlePing = useCallback(async (id: string) => {
    setPingingIds(prev => ({ ...prev, [id]: true }));
    try {
      const ms = await api.pingConnection(id);
      setPings(prev => ({ ...prev, [id]: ms }));
    } catch (err) {
      console.error('Failed to ping:', err);
      setPings(prev => ({ ...prev, [id]: -1 }));
    } finally {
      setPingingIds(prev => ({ ...prev, [id]: false }));
    }
  }, []);

  const handlePingAll = useCallback(async () => {
    if (isPingingAll || connections.length === 0) return;
    setIsPingingAll(true);
    const newPinging: Record<string, boolean> = {};
    connections.forEach(c => {
      newPinging[c.id] = true;
    });
    setPingingIds(newPinging);

    try {
      const res = await api.pingAll();
      setPings(prev => ({ ...prev, ...res }));
    } catch (err) {
      console.error('Failed to ping all:', err);
      showToast('Failed to ping some profiles', 'error');
    } finally {
      setIsPingingAll(false);
      setPingingIds({});
    }
  }, [connections, isPingingAll, showToast]);

  const isSelectedActive = activeStats?.id === selectedConnection?.id && selectedConnection?.active;
  const displayBytesRead = isSelectedActive ? activeStats.bytesRead : (selectedConnection?.bytesRead ?? 0);
  const displayBytesWritten = isSelectedActive ? activeStats.bytesWritten : (selectedConnection?.bytesWritten ?? 0);
  const displayTotalBytes = isSelectedActive
    ? (activeStats.totalBytes ?? (activeStats.bytesRead + activeStats.bytesWritten))
    : (selectedConnection?.totalBytes ?? ((selectedConnection?.bytesRead ?? 0) + (selectedConnection?.bytesWritten ?? 0)));

  const handleClearStuckTun = async () => {
    setIsClearingTun(true);
    try {
      await api.clearStuckTun();
      await loadConnections();
      setActiveStats(null);
      showToast('Stuck TUN connection and routing rules cleared!', 'success');
    } catch (err: any) {
      showToast('Failed to clear TUN: ' + (err?.message || String(err)), 'error');
    } finally {
      setIsClearingTun(false);
    }
  };

  const handleDragStart = (e: React.DragEvent, index: number) => {
    setDraggedIndex(index);
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', String(index));

    const targetItem = connections[index];
    if (targetItem && e.dataTransfer.setDragImage) {
      // Create a compact custom drag ghost to prevent WebKit HiDPI ballooning
      const ghost = document.createElement('div');
      ghost.style.position = 'fixed';
      ghost.style.top = '-9999px';
      ghost.style.left = '-9999px';
      ghost.style.zIndex = '9999';
      ghost.style.display = 'flex';
      ghost.style.alignItems = 'center';
      ghost.style.gap = '8px';
      ghost.style.padding = '8px 14px';
      ghost.style.background = '#0f172a';
      ghost.style.border = '1px solid #6366f1';
      ghost.style.borderRadius = '12px';
      ghost.style.color = '#f8fafc';
      ghost.style.fontSize = '12px';
      ghost.style.fontWeight = '600';
      ghost.style.boxShadow = '0 10px 25px -5px rgba(0, 0, 0, 0.5)';
      ghost.style.whiteSpace = 'nowrap';
      ghost.style.pointerEvents = 'none';

      const dot = document.createElement('span');
      dot.style.width = '8px';
      dot.style.height = '8px';
      dot.style.borderRadius = '50%';
      dot.style.backgroundColor = targetItem.active ? '#34d399' : '#818cf8';
      ghost.appendChild(dot);

      const label = document.createElement('span');
      label.textContent = targetItem.label;
      ghost.appendChild(label);

      document.body.appendChild(ghost);
      e.dataTransfer.setDragImage(ghost, 20, 20);

      setTimeout(() => {
        if (document.body.contains(ghost)) {
          document.body.removeChild(ghost);
        }
      }, 0);
    }
  };

  const handleDragOver = (e: React.DragEvent, index: number) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
    if (dragOverIndex !== index) {
      setDragOverIndex(index);
    }
  };

  const handleDrop = async (e: React.DragEvent, targetIndex: number) => {
    e.preventDefault();
    const sourceIndex = draggedIndex;
    setDraggedIndex(null);
    setDragOverIndex(null);

    if (sourceIndex === null || sourceIndex === targetIndex) {
      return;
    }

    await handleReorder(sourceIndex, targetIndex);
  };

  const handleDragEnd = () => {
    setDraggedIndex(null);
    setDragOverIndex(null);
  };

  return (
    <div className="flex flex-col h-screen w-screen bg-slate-950 text-slate-100 font-sans overflow-hidden select-none">
      {/* Toast Notification */}
      {toast && (
        <div className="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 flex items-center gap-2.5 px-4 py-2.5 rounded-xl shadow-2xl text-xs font-medium border backdrop-blur-md animate-in slide-in-from-bottom-3 duration-200 transition-all bg-slate-900/95 border-slate-700/80 text-slate-200 max-w-md">
          {toast.type === 'success' && <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />}
          {toast.type === 'error' && <AlertCircle className="w-4 h-4 text-rose-400 shrink-0" />}
          {toast.type === 'info' && <Info className="w-4 h-4 text-indigo-400 shrink-0" />}
          <span className="truncate">{toast.message}</span>
        </div>
      )}

      {/* Header Bar */}
      <header className="h-16 px-5 border-b border-slate-800 bg-slate-900/95 flex items-center justify-between shrink-0 relative">
        {/* Left: Brand, Mode Switcher & Connect / Disconnect */}
        <div className="flex items-center gap-4 min-w-0">
          <div className="flex items-center gap-3 shrink-0">
            <img
              src="/icon.png"
              alt="Kite"
              className="w-9 h-9 rounded-xl shadow-md shadow-indigo-600/30 border border-slate-700/50 object-cover"
            />
            <div className="flex flex-col">
              <h1 className="text-base font-bold tracking-tight text-white flex items-center gap-2 leading-none">
                Kite
                <span className="text-[10px] font-mono font-normal text-indigo-400 bg-indigo-500/10 px-1.5 py-0.5 rounded-md border border-indigo-500/20">
                  XRay
                </span>
              </h1>
              <span className="text-[10px] font-mono text-slate-400 mt-1 leading-none">
                {appInfo?.version ? `v${appInfo.version}` : 'v1.4.2'}
              </span>
            </div>
          </div>

          {/* Desktop Routing controls: hidden on small width */}
          <div className="hidden md:flex items-center gap-3">
            <div className="h-5 w-px bg-slate-800/80 shrink-0" />

            {/* Settings Button on the left of TPBS */}
            <button
              onClick={() => handleOpenSettings()}
              className="flex items-center justify-center w-7 h-7 rounded-full bg-slate-900 hover:bg-slate-800 text-slate-400 hover:text-indigo-400 border border-slate-800 hover:border-slate-700 transition-all shadow-xs active:scale-95 cursor-pointer shrink-0"
              title="Routing Mode & Hotkey Settings"
            >
              <Settings className="w-3.5 h-3.5" />
            </button>

            {/* Mode Switcher Segmented Control */}
            <div className="flex items-center bg-slate-900 p-0.5 rounded-full border border-slate-800 shadow-inner shrink-0">
              <button
                onClick={() => handleModeChange('tunnel')}
                className={`flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium transition-all cursor-pointer ${
                  tunnelMode === 'tunnel' || tunnelMode === 'system'
                    ? 'bg-indigo-600 text-white shadow-xs'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
                title="System Tunnel: Virtual TUN adapter routing network traffic with custom IP & DNS"
              >
                <Globe className="w-3.5 h-3.5" />
                <span>Tunnel</span>
              </button>

              <button
                onClick={() => handleModeChange('proxy')}
                className={`flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium transition-all cursor-pointer ${
                  tunnelMode === 'proxy'
                    ? 'bg-indigo-600 text-white shadow-xs'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
                title="System Proxy: System-wide proxy via WinINet/desktop settings without TUN driver"
              >
                <Network className="w-3.5 h-3.5" />
                <span>Proxy</span>
              </button>

              <button
                onClick={() => handleModeChange('bridge')}
                className={`flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium transition-all cursor-pointer ${
                  tunnelMode === 'bridge' || tunnelMode === 'per_app'
                    ? 'bg-indigo-600 text-white shadow-xs'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
                title="Bridge: Match executables via wildcards/regex with rules table and launcher"
              >
                <Layers className="w-3.5 h-3.5" />
                <span>Bridge</span>
              </button>
            </div>

            {/* Smart Connect / Disconnect Button */}
            {activeConnection ? (
              <div className="flex items-center gap-2 px-3.5 py-1 rounded-full bg-slate-900 border border-emerald-500/30 shadow-inner shrink-0">
                <span className="relative flex h-2 w-2">
                  <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-400 shadow-[0_0_8px_#34d399]" />
                </span>
                <span className="text-xs font-medium text-emerald-400 truncate max-w-[130px]" title={activeConnection.label}>
                  {activeConnection.label}
                </span>
                <button
                  onClick={() => handleDisconnect()}
                  disabled={disconnectingId !== null}
                  className="ml-1 p-1 rounded-full text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer"
                  title="Disconnect VPN"
                >
                  <Power className="w-3.5 h-3.5" />
                </button>
              </div>
            ) : connectingId !== null ? (
              <div className="flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-amber-500/10 border border-amber-500/30 text-amber-300 text-xs font-medium shrink-0">
                <Loader2 className="w-3.5 h-3.5 animate-spin text-amber-400" />
                <span>Connecting...</span>
              </div>
            ) : (
              <button
                onClick={handleSmartConnect}
                className="flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-indigo-600/15 hover:bg-indigo-600/25 border border-indigo-500/30 hover:border-indigo-500/60 text-indigo-300 hover:text-white transition-all shadow-xs active:scale-95 text-xs font-medium cursor-pointer shrink-0"
                title={targetConnection ? `Connect to "${targetConnection.label}"` : 'Connect to VPN'}
              >
                <Power className="w-3.5 h-3.5 text-indigo-400" />
                <span className="truncate max-w-[140px]">
                  {targetConnection ? `Connect (${targetConnection.label})` : 'Connect'}
                </span>
              </button>
            )}
          </div>
        </div>

        {/* Desktop Right: Primary Action + Minimal Utility Icons */}
        <div className="hidden md:flex items-center gap-2">
          <button
            onClick={() => {
              setEditItem(null);
              setIsAddModalOpen(true);
            }}
            className="flex items-center gap-1.5 px-3.5 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white rounded-xl text-xs font-semibold shadow-md shadow-indigo-600/20 active:scale-95 transition-all cursor-pointer"
          >
            <Plus className="w-4 h-4" />
            <span>Add Profile</span>
          </button>

          <button
            onClick={() => setCurrentTab(currentTab === 'about' ? 'connections' : 'about')}
            className={`p-2 rounded-xl border text-xs font-medium transition-all cursor-pointer ${
              currentTab === 'about'
                ? 'bg-indigo-600 border-indigo-500 text-white shadow-sm'
                : 'bg-slate-900 border-slate-800 text-slate-400 hover:text-slate-200 hover:border-slate-700'
            }`}
            title={currentTab === 'about' ? 'Back to Profiles' : 'About & Diagnostics'}
          >
            <Info className="w-4 h-4" />
          </button>

          <button
            onClick={() => api.quit()}
            className="p-2 rounded-xl bg-slate-900 hover:bg-rose-950/40 text-slate-400 hover:text-rose-400 border border-slate-800 hover:border-rose-500/30 transition-all cursor-pointer"
            title="Quit Application"
          >
            <LogOut className="w-4 h-4" />
          </button>
        </div>

        {/* Mobile Right: ConnectButton - AddProfileButton - MenuButton */}
        <div className="flex md:hidden items-center gap-1.5 shrink-0">
          {/* 1. ConnectButton: exact same size w-8 h-8, solid green when active without extra circle */}
          {activeConnection ? (
            <button
              onClick={() => handleDisconnect()}
              disabled={disconnectingId !== null}
              className="w-8 h-8 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white shadow-md shadow-emerald-600/25 border border-emerald-500/40 flex items-center justify-center transition-all cursor-pointer shrink-0"
              title={`Connected to "${activeConnection.label}". Click to disconnect.`}
            >
              <Power className="w-4 h-4" />
            </button>
          ) : connectingId !== null ? (
            <button
              disabled
              className="w-8 h-8 rounded-xl bg-amber-500/20 border border-amber-500/40 text-amber-400 flex items-center justify-center shrink-0"
              title="Connecting..."
            >
              <Loader2 className="w-4 h-4 animate-spin" />
            </button>
          ) : (
            <button
              onClick={handleSmartConnect}
              className="w-8 h-8 rounded-xl bg-indigo-600/20 hover:bg-indigo-600/30 border border-indigo-500/40 text-indigo-300 hover:text-white flex items-center justify-center transition-all cursor-pointer shrink-0"
              title={targetConnection ? `Connect to "${targetConnection.label}"` : 'Connect to VPN'}
            >
              <Power className="w-4 h-4" />
            </button>
          )}

          {/* 2. AddProfileButton */}
          <button
            onClick={() => {
              setEditItem(null);
              setIsAddModalOpen(true);
            }}
            className="w-8 h-8 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white shadow-md shadow-indigo-600/20 border border-indigo-500/40 flex items-center justify-center active:scale-95 transition-all cursor-pointer shrink-0"
            title="Add Profile"
          >
            <Plus className="w-4 h-4" />
          </button>

          {/* 3. MenuButton */}
          <button
            onClick={() => setIsMobileMenuOpen((prev) => !prev)}
            className={`w-8 h-8 rounded-xl border flex items-center justify-center transition-all cursor-pointer shrink-0 ${
              isMobileMenuOpen
                ? 'bg-indigo-600 border-indigo-500 text-white'
                : 'bg-slate-900 border-slate-800 text-slate-300 hover:text-white hover:border-slate-700'
            }`}
            title="Menu"
          >
            {isMobileMenuOpen ? <X className="w-4 h-4" /> : <MoreVertical className="w-4 h-4" />}
          </button>
        </div>
      </header>

      {/* Mobile Menu Dropdown Backdrop & Popover */}
      {isMobileMenuOpen && (
        <>
          <div
            className="fixed inset-0 z-40 bg-black/40 backdrop-blur-xs md:hidden"
            onClick={() => setIsMobileMenuOpen(false)}
          />
          <div className="absolute top-16 right-3 z-50 w-72 bg-slate-900/95 border border-slate-800 rounded-2xl shadow-2xl p-3 flex flex-col gap-2.5 backdrop-blur-md md:hidden animate-in fade-in slide-in-from-top-2 duration-150">
            {/* Mode Switcher */}
            <div className="flex flex-col gap-1.5">
              <span className="text-[10px] font-semibold uppercase tracking-wider text-slate-400 px-1">
                Routing Mode
              </span>
              <div className="grid grid-cols-3 gap-1 bg-slate-950 p-1 rounded-xl border border-slate-800">
                <button
                  onClick={() => {
                    handleModeChange('tunnel');
                    setIsMobileMenuOpen(false);
                  }}
                  className={`flex items-center justify-center gap-1 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                    tunnelMode === 'tunnel' || tunnelMode === 'system'
                      ? 'bg-indigo-600 text-white shadow-xs'
                      : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  <Globe className="w-3 h-3" />
                  <span>Tunnel</span>
                </button>
                <button
                  onClick={() => {
                    handleModeChange('proxy');
                    setIsMobileMenuOpen(false);
                  }}
                  className={`flex items-center justify-center gap-1 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                    tunnelMode === 'proxy'
                      ? 'bg-indigo-600 text-white shadow-xs'
                      : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  <Network className="w-3 h-3" />
                  <span>Proxy</span>
                </button>
                <button
                  onClick={() => {
                    handleModeChange('bridge');
                    setIsMobileMenuOpen(false);
                  }}
                  className={`flex items-center justify-center gap-1 py-1.5 rounded-lg text-xs font-medium transition-all cursor-pointer ${
                    tunnelMode === 'bridge' || tunnelMode === 'per_app'
                      ? 'bg-indigo-600 text-white shadow-xs'
                      : 'text-slate-400 hover:text-slate-200'
                  }`}
                >
                  <Layers className="w-3 h-3" />
                  <span>Bridge</span>
                </button>
              </div>
            </div>

            <div className="h-px bg-slate-800/80 my-0.5" />

            {/* Quick Actions */}
            <button
              onClick={() => {
                setEditItem(null);
                setIsAddModalOpen(true);
                setIsMobileMenuOpen(false);
              }}
              className="flex items-center gap-2.5 px-3 py-2 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold shadow-md shadow-indigo-600/20 active:scale-95 transition-all cursor-pointer"
            >
              <Plus className="w-4 h-4" />
              <span>Add Profile</span>
            </button>

            <button
              onClick={() => {
                handleOpenSettings('general');
                setIsMobileMenuOpen(false);
              }}
              className="flex items-center gap-2.5 px-3 py-2 rounded-xl text-slate-300 hover:text-white hover:bg-slate-800 text-xs font-medium transition-colors cursor-pointer"
            >
              <Settings className="w-4 h-4 text-indigo-400" />
              <span>Settings & Shortcuts</span>
            </button>

            <button
              onClick={() => {
                setCurrentTab(currentTab === 'about' ? 'connections' : 'about');
                setIsMobileMenuOpen(false);
              }}
              className="flex items-center gap-2.5 px-3 py-2 rounded-xl text-slate-300 hover:text-white hover:bg-slate-800 text-xs font-medium transition-colors cursor-pointer"
            >
              <Info className="w-4 h-4 text-indigo-400" />
              <span>{currentTab === 'about' ? 'Profiles List' : 'About & Diagnostics'}</span>
            </button>

            <div className="h-px bg-slate-800/80 my-0.5" />

            <button
              onClick={() => {
                setIsMobileMenuOpen(false);
                api.quit();
              }}
              className="flex items-center gap-2.5 px-3 py-2 rounded-xl text-rose-400 hover:bg-rose-950/30 text-xs font-medium transition-colors cursor-pointer"
            >
              <LogOut className="w-4 h-4" />
              <span>Quit Kite</span>
            </button>
          </div>
        </>
      )}

      {/* Body Content */}
      <main className="flex-1 overflow-hidden">
        {currentTab === 'about' ? (
          <div className="h-full overflow-y-auto">
            <AboutView
              onResetTun={handleClearStuckTun}
              isResettingTun={isClearingTun}
              updateInfo={updateInfo}
              updateProgress={updateProgress}
              onTriggerUpdate={(info) => {
                setUpdateInfo(info);
                setIsUpdateModalOpen(true);
              }}
            />
          </div>
        ) : connections.length === 0 ? (
          /* Empty State */
          <div className="h-full flex flex-col items-center justify-center p-8 text-center">
            <div className="w-16 h-16 rounded-3xl bg-slate-900 border border-slate-800 flex items-center justify-center text-slate-500 mb-4 shadow-xl">
              <Server className="w-8 h-8" />
            </div>
            <h3 className="text-base font-semibold text-slate-200">No VPN Profiles Configured</h3>
            <p className="text-xs text-slate-400 max-w-sm mt-1.5 mb-6">
              Add your XRay connection link (VLESS, VMess, Trojan, etc.) to start routing your network traffic securely.
            </p>
            <button
              onClick={() => {
                setEditItem(null);
                setIsAddModalOpen(true);
              }}
              className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white font-medium text-xs shadow-lg shadow-indigo-600/25 active:scale-95 transition-all"
            >
              <Plus className="w-4 h-4" />
              <span>Add Your First Profile</span>
            </button>
          </div>
        ) : (
          /* Master-Detail Split Screen */
          <div className="grid grid-cols-12 h-full">
            {/* Left Column: Profiles List */}
            <div className="col-span-12 md:col-span-6 lg:col-span-5 md:border-r border-slate-800/80 overflow-y-auto p-4 flex flex-col gap-3">
              <div className="flex items-center justify-between px-1 mb-1">
                <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                  Available Profiles ({connections.length})
                </span>
                <div className="flex items-center gap-1.5">
                  <button
                    onClick={handleToggleCompactMode}
                    className={`h-[26px] w-[26px] flex items-center justify-center rounded-lg border text-xs transition-all cursor-pointer shrink-0 ${
                      isCompactMode
                        ? 'bg-indigo-600/20 border-indigo-500/40 text-indigo-300 hover:bg-indigo-600/30'
                        : 'bg-slate-800/60 border-slate-700/60 text-slate-400 hover:text-slate-200 hover:bg-slate-800'
                    }`}
                    title={isCompactMode ? 'Switch to Comfortable view' : 'Switch to Compact view'}
                  >
                    {isCompactMode ? <LayoutList className="w-3.5 h-3.5" /> : <List className="w-3.5 h-3.5" />}
                  </button>
                  <button
                    onClick={handlePingAll}
                    disabled={isPingingAll || connections.length === 0}
                    className="h-[26px] flex items-center gap-1.5 px-2.5 rounded-lg text-xs font-medium text-slate-300 hover:text-white bg-slate-800/60 hover:bg-slate-800 border border-slate-700/60 transition-all cursor-pointer disabled:opacity-50"
                    title="Ping all profiles"
                  >
                    <Activity className={`w-3.5 h-3.5 text-indigo-400 ${isPingingAll ? 'animate-spin' : ''}`} />
                    <span>{isPingingAll ? 'Pinging...' : 'Ping all'}</span>
                  </button>
                </div>
              </div>

              {groups.map((group) => {
                const isCollapsed = !!collapsedGroups[group.id];
                const isUpdating = !!updatingSubIds[group.id];

                return (
                  <div key={group.id} className="flex flex-col gap-2">
                    {/* Group Header */}
                    <div className="flex items-center justify-between px-3 py-1.5 rounded-xl bg-slate-900/80 border border-slate-800/80 select-none shadow-xs">
                      <button
                        type="button"
                        onClick={() => toggleGroupCollapse(group.id)}
                        className="flex items-center gap-2 text-left min-w-0 flex-1 hover:text-white transition-colors cursor-pointer"
                      >
                        <ChevronDown
                          className={`w-3.5 h-3.5 text-slate-400 shrink-0 transition-transform duration-150 ${
                            isCollapsed ? '-rotate-90' : ''
                          }`}
                        />
                        {group.type === 'local' ? (
                          <HardDrive className="w-3.5 h-3.5 text-indigo-400 shrink-0" />
                        ) : (
                          <Rss className="w-3.5 h-3.5 text-amber-400 shrink-0" />
                        )}
                        <span className="text-xs font-semibold text-slate-200 truncate">
                          {group.title}
                        </span>
                        <span className="px-1.5 py-0.2 rounded-full text-[10px] font-mono font-medium bg-slate-800 text-slate-400 border border-slate-700/50 shrink-0">
                          {group.items.length}
                        </span>
                      </button>

                      {/* Group Actions for subscriptions */}
                      {group.type === 'subscription' && (
                        <div className="flex items-center gap-1 shrink-0 ml-2">
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              handleUpdateSubscription(group.id);
                            }}
                            disabled={isUpdating}
                            title="Update subscription links"
                            className="p-1 rounded-md text-slate-400 hover:text-amber-400 hover:bg-slate-800 transition-colors cursor-pointer disabled:opacity-50"
                          >
                            <RotateCw className={`w-3 h-3 ${isUpdating ? 'animate-spin text-amber-400' : ''}`} />
                          </button>
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              handleDeleteSubscription(group.id, group.title);
                            }}
                            title="Delete subscription group"
                            className="p-1 rounded-md text-slate-400 hover:text-rose-400 hover:bg-slate-800 transition-colors cursor-pointer"
                          >
                            <Trash2 className="w-3 h-3" />
                          </button>
                        </div>
                      )}
                    </div>

                    {/* Group Items */}
                    {!isCollapsed && (
                      <div className="flex flex-col gap-2">
                        {group.items.length === 0 ? (
                          <div className="px-3 py-3 text-center text-slate-500 text-xs rounded-xl bg-slate-900/20 border border-dashed border-slate-800/60">
                            No profiles in this group
                          </div>
                        ) : (
                          group.items.map(({ item, globalIndex }) => {
                            const isCardActive = activeStats?.id === item.id && item.active;
                            const cardItem = {
                              ...(isCardActive
                                ? {
                                    ...item,
                                    bytesRead: activeStats.bytesRead,
                                    bytesWritten: activeStats.bytesWritten,
                                    totalBytes: activeStats.totalBytes ?? (activeStats.bytesRead + activeStats.bytesWritten),
                                  }
                                : item),
                              pingMs: pings[item.id],
                            };

                            return (
                              <ProfileCard
                                key={item.id}
                                connection={cardItem}
                                index={globalIndex}
                                isSelected={selectedConnection?.id === item.id}
                                onSelect={() => setSelectedId(item.id)}
                                onConnect={() => handleConnect(item.id)}
                                onEdit={() => {
                                  setEditItem(item);
                                  setIsAddModalOpen(true);
                                }}
                                onDelete={() => handleDelete(item.id)}
                                onResetTraffic={() => handleResetTraffic(item.id)}
                                onPing={() => handlePing(item.id)}
                                isPinging={!!pingingIds[item.id] || isPingingAll}
                                onDragStart={handleDragStart}
                                onDragOver={handleDragOver}
                                onDrop={handleDrop}
                                onDragEnd={handleDragEnd}
                                isDragging={draggedIndex === globalIndex}
                                isDragOver={dragOverIndex === globalIndex && draggedIndex !== globalIndex}
                                isConnecting={connectingId === item.id}
                                isDisconnecting={disconnectingId === item.id}
                                compact={isCompactMode}
                              />
                            );
                          })
                        )}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>

            {/* Right Column: Live Monitor & Config Details */}
            <div className="hidden md:flex md:col-span-6 lg:col-span-7 overflow-y-auto p-5 flex-col gap-4 bg-slate-950/40">
              {selectedConnection && (
                <>
                  {/* Selected Profile Header */}
                  <div className="flex items-center justify-between bg-slate-900/60 rounded-2xl p-4 border border-slate-800">
                    <div className="flex items-center gap-3 min-w-0">
                      <div className={`w-10 h-10 rounded-xl flex items-center justify-center font-bold text-white shadow-md ${
                        selectedConnection.active ? 'bg-emerald-600 shadow-emerald-600/20' : 'bg-slate-800'
                      }`}>
                        <Activity className="w-5 h-5" />
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <h2 className="text-base font-bold text-slate-100 truncate" title={selectedConnection.label}>
                            {selectedConnection.label}
                          </h2>
                          {selectedConnection.subscriptionId ? (
                            <span className="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-amber-500/10 text-amber-400 border border-amber-500/20 shrink-0">
                              {(() => {
                                const sub = subscriptions.find(s => s.id === selectedConnection.subscriptionId);
                                return sub ? getSubscriptionGroupName(sub) : 'Subscription';
                              })()}
                            </span>
                          ) : (
                            <span className="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 shrink-0">
                              Local
                            </span>
                          )}
                        </div>
                        <div className="text-xs font-mono text-slate-400 truncate mt-0.5">
                          {selectedConnection.address ? `${selectedConnection.address}:${selectedConnection.port}` : 'Local profile'}
                        </div>
                      </div>
                    </div>

                    <div className="flex flex-col items-end gap-2 shrink-0">
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => handlePing(selectedConnection.id)}
                          disabled={!!pingingIds[selectedConnection.id] || isPingingAll}
                          className={`flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-mono border cursor-pointer transition-all ${
                            pingingIds[selectedConnection.id] || isPingingAll
                              ? 'text-indigo-300 bg-indigo-950/40 border-indigo-700/50 animate-pulse'
                              : pings[selectedConnection.id] !== undefined && pings[selectedConnection.id] > 0
                              ? pings[selectedConnection.id] < 120
                                ? 'text-emerald-400 bg-emerald-950/30 border-emerald-800/40 hover:bg-emerald-900/40'
                                : pings[selectedConnection.id] < 250
                                ? 'text-amber-400 bg-amber-950/30 border-amber-800/40 hover:bg-amber-900/40'
                                : 'text-rose-400 bg-rose-950/30 border-rose-800/40 hover:bg-rose-900/40'
                              : pings[selectedConnection.id] === -1
                              ? 'text-rose-400 bg-rose-950/30 border-rose-800/40'
                              : 'text-slate-400 bg-slate-800/60 border-slate-700 hover:text-white'
                          }`}
                          title="Ping server"
                        >
                          <Activity className={`w-3.5 h-3.5 ${pingingIds[selectedConnection.id] || isPingingAll ? 'animate-spin' : ''}`} />
                          <span>
                            {pingingIds[selectedConnection.id] || isPingingAll
                              ? 'Pinging...'
                              : pings[selectedConnection.id] !== undefined && pings[selectedConnection.id] > 0
                              ? `${pings[selectedConnection.id]} ms`
                              : pings[selectedConnection.id] === -1
                              ? 'Timeout'
                              : 'Ping'}
                          </span>
                        </button>

                        <button
                          onClick={() => handleConnect(selectedConnection.id)}
                          disabled={connectingId === selectedConnection.id || disconnectingId === selectedConnection.id}
                          className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition-all shadow-md ${
                            connectingId === selectedConnection.id
                              ? 'bg-amber-500/20 text-amber-300 border border-amber-500/40 cursor-wait'
                              : disconnectingId === selectedConnection.id
                              ? 'bg-rose-500/20 text-rose-300 border border-rose-500/40 cursor-wait'
                              : selectedConnection.active
                              ? 'bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 border border-rose-500/40 active:scale-95'
                              : 'bg-emerald-600 hover:bg-emerald-500 text-white shadow-emerald-600/20 active:scale-95'
                          }`}
                        >
                          {connectingId === selectedConnection.id ? (
                            <>
                              <Loader2 className="w-4 h-4 animate-spin text-amber-400" />
                              <span>Connecting...</span>
                            </>
                          ) : disconnectingId === selectedConnection.id ? (
                            <>
                              <Loader2 className="w-4 h-4 animate-spin text-rose-400" />
                              <span>Disconnecting...</span>
                            </>
                          ) : selectedConnection.active ? (
                            <>
                              <Power className="w-4 h-4" />
                              <span>Disconnect</span>
                            </>
                          ) : (
                            <>
                              <Power className="w-4 h-4" />
                              <span>Connect</span>
                            </>
                          )}
                        </button>
                      </div>

                      <button
                        onClick={() => {
                          navigator.clipboard.writeText(selectedConnection.link);
                          showToast('Connection link copied to clipboard', 'success');
                        }}
                        className="flex items-center gap-1 px-2.5 py-1 text-[11px] font-medium text-slate-400 hover:text-slate-200 bg-slate-800/50 hover:bg-slate-800 border border-slate-700/50 hover:border-slate-600 rounded-lg transition-all active:scale-95 cursor-pointer"
                        title="Copy profile connection URL"
                      >
                        <Copy className="w-3 h-3" />
                        <span>Copy link</span>
                      </button>
                    </div>
                  </div>

                  {/* Cumulative Profile Traffic Card */}
                  <div className="bg-slate-900/60 rounded-2xl p-4 border border-slate-800 flex items-center justify-between shadow-xs">
                    <div className="flex items-center gap-6">
                      <div className="flex flex-col">
                        <span className="text-[10px] font-semibold uppercase tracking-wider text-slate-400">
                          Download
                        </span>
                        <span className="text-sm font-bold font-mono text-sky-400 mt-0.5 flex items-center gap-1">
                          <ArrowDown className="w-3.5 h-3.5" />
                          <span>{formatBytes(displayBytesWritten)}</span>
                        </span>
                      </div>

                      <div className="h-8 w-px bg-slate-800" />

                      <div className="flex flex-col">
                        <span className="text-[10px] font-semibold uppercase tracking-wider text-slate-400">
                          Upload
                        </span>
                        <span className="text-sm font-bold font-mono text-emerald-400 mt-0.5 flex items-center gap-1">
                          <ArrowUp className="w-3.5 h-3.5" />
                          <span>{formatBytes(displayBytesRead)}</span>
                        </span>
                      </div>

                      <div className="h-8 w-px bg-slate-800" />

                      <div className="flex flex-col">
                        <span className="text-[10px] font-semibold uppercase tracking-wider text-slate-400">
                          Total
                        </span>
                        <span className="text-sm font-bold font-mono text-slate-100 mt-0.5">
                          {formatBytes(displayTotalBytes)}
                        </span>
                      </div>
                    </div>

                    <button
                      onClick={() => handleResetTraffic(selectedConnection.id)}
                      className="flex items-center gap-1.5 px-3 py-1.5 bg-slate-800/90 hover:bg-amber-950/40 text-slate-300 hover:text-amber-300 border border-slate-700/60 hover:border-amber-500/40 rounded-xl text-xs font-medium transition-all shadow-xs active:scale-95"
                      title="Reset cumulative traffic statistics for this profile to 0"
                    >
                      <RotateCcw className="w-3.5 h-3.5 text-slate-400 group-hover:text-amber-300" />
                      <span>Reset Traffic</span>
                    </button>
                  </div>

                  {/* Real-time Network Traffic Chart */}
                  <div className="flex flex-col gap-1.5">
                    <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-400 px-1 flex items-center justify-between">
                      <span>Real-Time Traffic Monitor</span>
                      <span className="font-normal text-[11px] text-slate-500">60s rolling window</span>
                    </h3>
                    <NetworkChart
                      downloadHistory={activeStats?.id === selectedConnection.id ? activeStats.writeHistory : []}
                      uploadHistory={activeStats?.id === selectedConnection.id ? activeStats.readHistory : []}
                      currentDownload={activeStats?.id === selectedConnection.id ? activeStats.downloadSpeed : 0}
                      currentUpload={activeStats?.id === selectedConnection.id ? activeStats.uploadSpeed : 0}
                      height={130}
                    />
                  </div>
                </>
              )}
            </div>
          </div>
        )}
      </main>

      {/* Add / Edit Modal */}
      <AddEditModal
        isOpen={isAddModalOpen}
        onClose={() => {
          setIsAddModalOpen(false);
          setEditItem(null);
        }}
        onSuccess={() => {
          loadConnections();
          loadSubscriptions();
          showToast(editItem ? 'Profile updated' : 'Profile added successfully', 'success');
        }}
        editItem={editItem}
      />

      {/* Update Available Modal */}
      <UpdateModal
        isOpen={isUpdateModalOpen}
        updateInfo={updateInfo}
        progress={updateProgress}
        onClose={() => setIsUpdateModalOpen(false)}
        onInstall={handleInstallUpdate}
        onCancel={handleCancelUpdate}
        onSnooze={handleSnoozeUpdate}
        onSkipVersion={handleSkipVersion}
      />

      {/* Routing Mode Settings Modal (Tunnel / Proxy / Bridge) */}
      <ModeSettingsModal
        isOpen={isModeSettingsOpen}
        initialTab={settingsTab}
        onClose={() => setIsModeSettingsOpen(false)}
        onResetTun={handleClearStuckTun}
        isResettingTun={isClearingTun}
        isConnected={!!activeConnection}
        activeLabel={activeConnection?.label}
        onConnect={handleSmartConnect}
        showToast={showToast}
      />

      {/* Network Privileges Required Modal */}
      <PrivilegeModal
        isOpen={isPrivilegeModalOpen}
        command={privilegeInfo?.command || ''}
        os={privilegeInfo?.os}
        errorMessage={privilegeInfo?.error}
        onClose={() => setIsPrivilegeModalOpen(false)}
        onCheckAgain={handleCheckPrivilegesAgain}
        onGrantWithPkexec={handleGrantWithPkexec}
      />
    </div>
  );
}

export default App;
