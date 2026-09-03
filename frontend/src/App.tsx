import { useState, useEffect, useCallback } from 'react';
import {
  Plus,
  Power,
  Activity,
  Info,
  AlertCircle,
  CheckCircle2,
  RefreshCw,
  Server,
  RotateCcw,
  Loader2,
  LogOut,
  ArrowUp,
  ArrowDown,
  Globe,
  Rocket
} from 'lucide-react';
import { api } from './api/wails';
import type { ConnectionDTO, TunnelMode, InstalledApp, ReleaseInfo, UpdateProgress } from './types';
import { ProfileCard } from './components/ProfileCard';
import { NetworkChart } from './components/NetworkChart';
import { ConfigDetails } from './components/ConfigDetails';
import { AddEditModal } from './components/AddEditModal';
import { SelectAppModal } from './components/SelectAppModal';
import { UpdateModal } from './components/UpdateModal';
import { AboutView } from './components/AboutView';
import { PerAppView } from './components/PerAppView';
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

  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [editItem, setEditItem] = useState<ConnectionDTO | null>(null);
  const [currentTab, setCurrentTab] = useState<'connections' | 'about'>('connections');
  const [tunnelMode, setTunnelMode] = useState<TunnelMode>('system');
  const [isClearingTun, setIsClearingTun] = useState(false);
  const [isLaunchingExe, setIsLaunchingExe] = useState(false);
  const [isSelectAppModalOpen, setIsSelectAppModalOpen] = useState(false);
  const [targetConnForApp, setTargetConnForApp] = useState<string | null>(null);
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

  useEffect(() => {
    const unsubProgress = api.onUpdateProgress(prog => {
      setUpdateProgress(prog);
    });

    const timer = setTimeout(async () => {
      try {
        const info = await api.checkForUpdate();
        if (info && info.available) {
          setUpdateInfo(info);
          setIsUpdateModalOpen(true);
        }
      } catch (err) {
        console.debug('Background update check:', err);
      }
    }, 2000);

    return () => {
      unsubProgress();
      clearTimeout(timer);
    };
  }, []);

  const handleInstallUpdate = async (assetUrl: string, releaseUrl: string) => {
    try {
      await api.installUpdate(assetUrl, releaseUrl);
    } catch (err: any) {
      showToast(`Update error: ${err?.message || err}`, 'error');
    }
  };

  useEffect(() => {
    api.getTunnelMode().then(m => setTunnelMode((m as TunnelMode) || 'system'));

    const unsubMode = api.onModeChanged(m => {
      setTunnelMode((m as TunnelMode) || 'system');
    });

    return () => {
      unsubMode();
    };
  }, []);

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
      showToast(
        `Switched to ${newMode === 'per_app' ? 'App' : 'System'}`,
        'info'
      );
    } catch (err: any) {
      showToast(`Failed to switch mode: ${err?.message || err}`, 'error');
    }
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

  const handleOpenAppSelector = (targetConnId?: string) => {
    const connId = targetConnId || selectedConnection?.id || activeConnection?.id || connections[0]?.id;
    if (!connId) {
      showToast('Please add or select a profile first', 'error');
      setIsAddModalOpen(true);
      return;
    }
    setTargetConnForApp(connId);
    setIsSelectAppModalOpen(true);
  };

  const handleAppSelected = async (app: InstalledApp) => {
    setIsSelectAppModalOpen(false);
    const connId = targetConnForApp || selectedConnection?.id || activeConnection?.id || connections[0]?.id;
    if (!connId) return;

    try {
      setIsLaunchingExe(true);
      showToast(`Routing ${app.name} through Kite proxy...`, 'info');

      if (tunnelMode !== 'per_app') {
        await handleModeChange('per_app');
      }

      await api.launchAndRouteApp(connId, app.exePath);
      showToast(`Started proxy and launched ${app.name}!`, 'success');
    } catch (err: any) {
      showToast(`Failed to route ${app.name}: ${err?.message || err}`, 'error');
    } finally {
      setIsLaunchingExe(false);
    }
  };

  const handleBrowseManual = async () => {
    setIsSelectAppModalOpen(false);
    await handleSelectAndRouteExe(targetConnForApp || undefined);
  };

  const handleSelectAndRouteExe = async (targetConnId?: string) => {
    const connId = targetConnId || selectedConnection?.id || activeConnection?.id || connections[0]?.id;
    if (!connId) {
      showToast('Please add or select a profile first', 'error');
      setIsAddModalOpen(true);
      return;
    }

    try {
      const selectedPath = await api.selectExecutableDialog();
      if (!selectedPath) return;

      setIsLaunchingExe(true);
      const appName = selectedPath.split(/[/\\]/).pop() || selectedPath;
      showToast(`Routing ${appName} through Kite proxy...`, 'info');

      if (tunnelMode !== 'per_app') {
        await handleModeChange('per_app');
      }

      await api.launchAndRouteApp(connId, selectedPath);
      showToast(`Started proxy and launched ${appName}!`, 'success');
    } catch (err: any) {
      showToast(`Failed to route app: ${err?.message || err}`, 'error');
    } finally {
      setIsLaunchingExe(false);
    }
  };

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
      <header className="h-16 px-5 border-b border-slate-800/80 bg-slate-900/50 backdrop-blur-md flex items-center justify-between shrink-0">
        {/* Left: Brand */}
        <div className="flex items-center gap-3">
          <img
            src="/icon.png"
            alt="Kite"
            className="w-9 h-9 rounded-xl shadow-md shadow-indigo-600/30 border border-slate-700/50 object-cover"
          />
          <h1 className="text-base font-bold tracking-tight text-white flex items-center gap-2">
            Kite
            <span className="text-[10px] font-mono font-normal text-indigo-400 bg-indigo-500/10 px-1.5 py-0.5 rounded-md border border-indigo-500/20">
              XRay
            </span>
          </h1>
        </div>

        {/* Center: Mode Switcher & Smart Connect / Disconnect */}
        <div className="flex items-center gap-3">
          {/* Mode Switcher Segmented Control */}
          <div className="flex items-center bg-slate-900 p-1 rounded-full border border-slate-800 shadow-inner">
            <button
              onClick={() => handleModeChange('system')}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium transition-all cursor-pointer ${
                tunnelMode === 'system'
                  ? 'bg-indigo-600 text-white shadow-xs'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
              title="System-Wide: Routes 100% of computer network traffic through VPN"
            >
              <Globe className="w-3.5 h-3.5" />
              <span>System</span>
            </button>
            <button
              onClick={() => handleModeChange('per_app')}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium transition-all cursor-pointer ${
                tunnelMode === 'per_app'
                  ? 'bg-indigo-600 text-white shadow-xs'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
              title="App Tunnel: Inbound SOCKS5 & HTTP proxies for selected apps"
            >
              <Rocket className="w-3.5 h-3.5" />
              <span>App</span>
            </button>
          </div>

          {/* Smart Connect / Disconnect Button */}
          {activeConnection ? (
            <div className="flex items-center gap-2 px-3.5 py-1 rounded-full bg-slate-900 border border-emerald-500/30 shadow-inner">
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
                <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500" />
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
            <div className="flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-amber-500/10 border border-amber-500/30 text-amber-300 text-xs font-medium">
              <Loader2 className="w-3.5 h-3.5 animate-spin text-amber-400" />
              <span>Connecting...</span>
            </div>
          ) : (
            <button
              onClick={handleSmartConnect}
              className="flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-indigo-600/15 hover:bg-indigo-600/25 border border-indigo-500/30 hover:border-indigo-500/60 text-indigo-300 hover:text-white transition-all shadow-xs active:scale-95 text-xs font-medium cursor-pointer"
              title={targetConnection ? `Connect to "${targetConnection.label}"` : 'Connect to VPN'}
            >
              <Power className="w-3.5 h-3.5 text-indigo-400" />
              <span className="truncate max-w-[140px]">
                {targetConnection ? `Connect (${targetConnection.label})` : 'Connect'}
              </span>
            </button>
          )}
        </div>

        {/* Right: Primary Action + Minimal Utility Icons */}
        <div className="flex items-center gap-2">
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
      </header>

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
        ) : tunnelMode === 'per_app' ? (
          <div className="h-full overflow-y-auto">
            <PerAppView
              isConnected={!!activeConnection}
              activeLabel={activeConnection?.label}
              onConnect={handleSmartConnect}
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
            <div className="col-span-6 lg:col-span-5 border-r border-slate-800/80 overflow-y-auto p-4 flex flex-col gap-3">
              <div className="flex items-center justify-between px-1 mb-1">
                <span className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                  Available Profiles ({connections.length})
                </span>
                <button
                  onClick={loadConnections}
                  className="p-1 rounded-md text-slate-500 hover:text-slate-300 transition-colors"
                  title="Refresh list"
                >
                  <RefreshCw className="w-3.5 h-3.5" />
                </button>
              </div>

              {connections.map((item, idx) => {
                const isCardActive = activeStats?.id === item.id && item.active;
                const cardItem = isCardActive
                  ? {
                      ...item,
                      bytesRead: activeStats.bytesRead,
                      bytesWritten: activeStats.bytesWritten,
                      totalBytes: activeStats.totalBytes ?? (activeStats.bytesRead + activeStats.bytesWritten),
                    }
                  : item;
                return (
                  <ProfileCard
                    key={item.id}
                    connection={cardItem}
                    index={idx}
                    isSelected={selectedConnection?.id === item.id}
                    onSelect={() => setSelectedId(item.id)}
                    onConnect={() => handleConnect(item.id)}
                    onEdit={() => {
                      setEditItem(item);
                      setIsAddModalOpen(true);
                    }}
                    onDelete={() => handleDelete(item.id)}
                    onResetTraffic={() => handleResetTraffic(item.id)}
                    onDragStart={handleDragStart}
                    onDragOver={handleDragOver}
                    onDrop={handleDrop}
                    onDragEnd={handleDragEnd}
                    isDragging={draggedIndex === idx}
                    isDragOver={dragOverIndex === idx && draggedIndex !== idx}
                    isConnecting={connectingId === item.id}
                    isDisconnecting={disconnectingId === item.id}
                  />
                );
              })}
            </div>

            {/* Right Column: Live Monitor & Config Details */}
            <div className="col-span-6 lg:col-span-7 overflow-y-auto p-5 flex flex-col gap-4 bg-slate-950/40">
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
                          {selectedConnection.active && (
                            <span className="px-2 py-0.5 text-[10px] font-semibold rounded-full bg-emerald-500/20 text-emerald-400 border border-emerald-500/30">
                              Connected
                            </span>
                          )}
                        </div>
                        <div className="text-xs font-mono text-slate-400 truncate mt-0.5">
                          {selectedConnection.address ? `${selectedConnection.address}:${selectedConnection.port}` : 'Local profile'}
                        </div>
                      </div>
                    </div>

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

                  {/* Config Parameters Table & Per-App Action */}
                  <ConfigDetails
                    connection={selectedConnection}
                    tunnelMode={tunnelMode}
                    onSelectExecutable={() => handleOpenAppSelector(selectedConnection.id)}
                    isLaunchingApp={isLaunchingExe}
                  />
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
          showToast(editItem ? 'Profile updated' : 'Profile added successfully', 'success');
        }}
        editItem={editItem}
      />

      {/* Select Installed App Modal */}
      <SelectAppModal
        isOpen={isSelectAppModalOpen}
        onClose={() => setIsSelectAppModalOpen(false)}
        onSelectApp={handleAppSelected}
        onBrowseManual={handleBrowseManual}
        isLaunching={isLaunchingExe}
      />

      {/* Update Available Modal */}
      <UpdateModal
        isOpen={isUpdateModalOpen}
        updateInfo={updateInfo}
        progress={updateProgress}
        onClose={() => setIsUpdateModalOpen(false)}
        onInstall={handleInstallUpdate}
      />
    </div>
  );
}

export default App;
