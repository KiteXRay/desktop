import React, { useState, useEffect, useMemo } from 'react';
import {
  X,
  Search,
  FolderOpen,
  Loader2,
  Rocket,
  AppWindow,
  ArrowRight
} from 'lucide-react';
import { api } from '../api/wails';
import type { InstalledApp } from '../types';

interface SelectAppModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSelectApp: (app: InstalledApp) => void;
  onBrowseManual: () => void;
  isLaunching?: boolean;
}

export const SelectAppModal: React.FC<SelectAppModalProps> = ({
  isOpen,
  onClose,
  onSelectApp,
  onBrowseManual,
  isLaunching = false,
}) => {
  const [apps, setApps] = useState<InstalledApp[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  useEffect(() => {
    if (isOpen) {
      setSearchQuery('');
      setIsLoading(true);
      api.getInstalledApps()
        .then(list => setApps(list))
        .catch(err => console.error('Failed to scan installed apps:', err))
        .finally(() => setIsLoading(false));
    }
  }, [isOpen]);

  const filteredApps = useMemo(() => {
    if (!searchQuery.trim()) return apps;
    const q = searchQuery.toLowerCase().trim();
    return apps.filter(
      app =>
        app.name.toLowerCase().includes(q) ||
        app.exePath.toLowerCase().includes(q) ||
        (app.description && app.description.toLowerCase().includes(q))
    );
  }, [apps, searchQuery]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/75 backdrop-blur-xs animate-in fade-in duration-200">
      <div
        className="relative w-full max-w-xl bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[85vh] animate-in zoom-in-95 duration-150"
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-slate-800/80 bg-slate-900/90">
          <div className="flex items-center gap-3">
            <div className="w-9 h-9 rounded-xl bg-indigo-600/20 border border-indigo-500/30 flex items-center justify-center text-indigo-400 shadow-xs">
              <Rocket className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-sm font-bold text-slate-100 flex items-center gap-2">
                <span>Select App</span>
                <span className="px-2 py-0.5 text-[10px] font-semibold bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 rounded-full">
                  App
                </span>
              </h3>
              <p className="text-xs text-slate-400 mt-0.5">
                Choose an installed app or select any executable from disk
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            disabled={isLaunching}
            className="p-1.5 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors cursor-pointer"
            title="Close dialog"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Search & Manual Browse Header Bar */}
        <div className="p-4 border-b border-slate-800/60 bg-slate-950/40 flex flex-col gap-2.5">
          <div className="relative flex items-center">
            <Search className="absolute left-3 w-4 h-4 text-slate-500 pointer-events-none" />
            <input
              type="text"
              value={searchQuery}
              onChange={e => setSearchQuery(e.target.value)}
              placeholder="Search installed applications..."
              autoFocus
              className="w-full bg-slate-900/90 border border-slate-800 rounded-xl pl-9 pr-8 py-2 text-xs text-slate-100 placeholder-slate-500 focus:outline-hidden focus:border-indigo-500/60 focus:ring-1 focus:ring-indigo-500/30 transition-all"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery('')}
                className="absolute right-2.5 p-0.5 text-slate-500 hover:text-slate-300 rounded cursor-pointer"
                title="Clear search"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          <div className="flex items-center justify-between text-xs">
            <span className="text-slate-500 text-[11px]">
              {isLoading ? (
                'Scanning applications...'
              ) : (
                <>Found <span className="font-semibold text-slate-300">{apps.length}</span> installed apps</>
              )}
            </span>
            <button
              onClick={() => {
                onClose();
                onBrowseManual();
              }}
              disabled={isLaunching}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-indigo-600/15 hover:bg-indigo-600/25 border border-indigo-500/30 text-indigo-300 hover:text-indigo-100 text-xs font-medium transition-all cursor-pointer active:scale-95"
              title="Select an executable file (.exe or binary) directly from your file system"
            >
              <FolderOpen className="w-3.5 h-3.5 text-indigo-400" />
              <span>Browse Executable File...</span>
            </button>
          </div>
        </div>

        {/* App List Area */}
        <div className="flex-1 overflow-y-auto p-2 space-y-1 min-h-[260px] max-h-[420px]">
          {isLoading ? (
            <div className="flex flex-col items-center justify-center py-16 text-center text-slate-500 gap-3">
              <Loader2 className="w-7 h-7 animate-spin text-indigo-400" />
              <div className="text-xs font-medium text-slate-400">Scanning installed applications...</div>
              <div className="text-[11px] text-slate-600">Reading system desktop entries and registries</div>
            </div>
          ) : filteredApps.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center text-slate-500 gap-3">
              <div className="w-12 h-12 rounded-2xl bg-slate-800/60 flex items-center justify-center text-slate-400">
                <AppWindow className="w-6 h-6" />
              </div>
              <div className="text-xs font-medium text-slate-300">
                {searchQuery ? `No applications found matching "${searchQuery}"` : 'No applications detected'}
              </div>
              <p className="text-[11px] text-slate-500 max-w-xs">
                Your application might be installed in a custom location. Choose its executable directly.
              </p>
              <button
                onClick={() => {
                  onClose();
                  onBrowseManual();
                }}
                className="mt-1 flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold shadow-md transition-all cursor-pointer"
              >
                <FolderOpen className="w-3.5 h-3.5" />
                <span>Select Executable Manually</span>
              </button>
            </div>
          ) : (
            filteredApps.map(app => (
              <div
                key={app.exePath}
                onClick={() => onSelectApp(app)}
                className="group flex items-center justify-between p-2.5 rounded-xl bg-slate-900/40 hover:bg-indigo-600/10 border border-slate-800/40 hover:border-indigo-500/30 transition-all cursor-pointer"
              >
                <div className="flex items-center gap-3 min-w-0 pr-3">
                  <div className="w-8 h-8 rounded-lg bg-slate-800 group-hover:bg-indigo-600/20 flex items-center justify-center text-slate-300 group-hover:text-indigo-300 font-bold text-xs uppercase shrink-0 transition-colors border border-slate-700/40 group-hover:border-indigo-500/30 overflow-hidden">
                    {app.icon ? (
                      <img src={app.icon} alt={app.name} className="w-full h-full object-contain p-0.5" />
                    ) : (
                      app.name.charAt(0)
                    )}
                  </div>
                  <div className="min-w-0">
                    <div className="text-xs font-semibold text-slate-200 group-hover:text-white truncate">
                      {app.name}
                    </div>
                    <div className="text-[10px] font-mono text-slate-500 group-hover:text-slate-400 truncate mt-0.5" title={app.exePath}>
                      {app.exePath}
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-1.5 text-slate-500 group-hover:text-indigo-300 shrink-0 text-xs font-medium opacity-75 group-hover:opacity-100 transition-all">
                  <span className="text-[11px] hidden group-hover:inline">Tunnel</span>
                  <ArrowRight className="w-3.5 h-3.5 transform group-hover:translate-x-0.5 transition-transform" />
                </div>
              </div>
            ))
          )}
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between px-5 py-3 border-t border-slate-800/80 bg-slate-900/90 text-xs">
          <span className="text-slate-500 text-[11px]">
            Showing {filteredApps.length} of {apps.length} apps
          </span>
          <div className="flex items-center gap-2">
            <button
              onClick={onClose}
              disabled={isLaunching}
              className="px-3.5 py-1.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white text-xs font-medium transition-all cursor-pointer"
            >
              Cancel
            </button>
            <button
              onClick={() => {
                onClose();
                onBrowseManual();
              }}
              disabled={isLaunching}
              className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-xl bg-slate-800 hover:bg-indigo-600/20 text-indigo-300 hover:text-indigo-100 border border-slate-700/60 hover:border-indigo-500/40 text-xs font-medium transition-all cursor-pointer"
            >
              <FolderOpen className="w-3.5 h-3.5" />
              <span>Choose from Disk...</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
