import React from 'react';
import { Download, Sparkles, ExternalLink, X, AlertCircle, Loader2 } from 'lucide-react';
import type { ReleaseInfo, UpdateProgress } from '../types';

interface UpdateModalProps {
  isOpen: boolean;
  updateInfo: ReleaseInfo | null;
  progress: UpdateProgress | null;
  onClose: () => void;
  onInstall: (assetUrl: string, releaseUrl: string) => void;
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

export const UpdateModal: React.FC<UpdateModalProps> = ({
  isOpen,
  updateInfo,
  progress,
  onClose,
  onInstall,
}) => {
  if (!isOpen || !updateInfo) return null;

  const isDownloading = progress?.status === 'downloading';
  const isApplying = progress?.status === 'applying';
  const isCompleted = progress?.status === 'completed';
  const isError = progress?.status === 'error';
  const isBusy = isDownloading || isApplying;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/75 backdrop-blur-xs animate-in fade-in duration-150">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md shadow-2xl overflow-hidden flex flex-col gap-4 p-5 animate-in zoom-in-95 duration-200">
        
        {/* Header */}
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-indigo-500/10 border border-indigo-500/20 text-indigo-400 flex items-center justify-center shrink-0">
              <Sparkles className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-bold text-slate-100">Update Available</h3>
              <p className="text-xs text-slate-400">
                A newer version of Kite is ready
              </p>
            </div>
          </div>
          {!isBusy && (
            <button
              onClick={onClose}
              className="p-1 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors cursor-pointer"
            >
              <X className="w-4 h-4" />
            </button>
          )}
        </div>

        {/* Version Badge Comparison */}
        <div className="flex items-center justify-between px-3.5 py-2.5 bg-slate-950/60 border border-slate-800/80 rounded-xl text-xs">
          <span className="text-slate-400">
            Current: <span className="font-semibold text-slate-300">v{updateInfo.currentVersion}</span>
          </span>
          <span className="text-slate-600 font-bold">→</span>
          <span className="text-indigo-400 font-semibold bg-indigo-500/10 px-2 py-0.5 rounded-md border border-indigo-500/20">
            Latest: {updateInfo.latestVersion}
          </span>
        </div>

        {/* Release Notes Preview */}
        {(updateInfo.releaseTitle || updateInfo.releaseNotes) && (
          <div className="flex flex-col gap-1.5">
            <span className="text-xs font-medium text-slate-400">What's New:</span>
            <div className="max-h-36 overflow-y-auto rounded-xl bg-slate-950/60 border border-slate-800/80 p-3 text-xs text-slate-300 whitespace-pre-wrap leading-relaxed">
              {updateInfo.releaseTitle && (
                <div className="font-semibold text-slate-200 mb-1.5">{updateInfo.releaseTitle}</div>
              )}
              {updateInfo.releaseNotes || 'No changelog provided.'}
            </div>
          </div>
        )}

        {/* Download / Progress Status */}
        {isDownloading && (
          <div className="flex flex-col gap-2 p-3 bg-slate-950/60 border border-slate-800/80 rounded-xl">
            <div className="flex items-center justify-between text-xs text-slate-300">
              <span className="flex items-center gap-1.5 text-indigo-400 font-medium">
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
                Downloading update...
              </span>
              <span className="font-mono text-slate-400">
                {progress.percentage.toFixed(0)}%
              </span>
            </div>
            <div className="w-full bg-slate-800 rounded-full h-2 overflow-hidden">
              <div
                className="bg-indigo-500 h-full transition-all duration-200 rounded-full"
                style={{ width: `${Math.max(5, progress.percentage)}%` }}
              />
            </div>
            {progress.total > 0 && (
              <div className="text-[11px] text-slate-400 text-right font-mono">
                {formatBytes(progress.downloaded)} / {formatBytes(progress.total)}
              </div>
            )}
          </div>
        )}

        {isApplying && (
          <div className="flex items-center gap-2 p-3 bg-indigo-500/10 border border-indigo-500/20 text-indigo-300 rounded-xl text-xs">
            <Loader2 className="w-4 h-4 animate-spin shrink-0" />
            <span>Applying update and preparing restart...</span>
          </div>
        )}

        {isCompleted && (
          <div className="flex items-center gap-2 p-3 bg-emerald-500/10 border border-emerald-500/20 text-emerald-300 rounded-xl text-xs">
            <span>Update installed successfully! Restarting application...</span>
          </div>
        )}

        {isError && (
          <div className="flex items-start gap-2 p-3 bg-rose-500/10 border border-rose-500/20 text-rose-300 rounded-xl text-xs">
            <AlertCircle className="w-4 h-4 shrink-0 mt-0.5 text-rose-400" />
            <div className="flex-1">
              <div className="font-semibold">Failed to install update</div>
              <div className="text-rose-400/80 text-[11px] mt-0.5">{progress.error}</div>
            </div>
          </div>
        )}

        {/* Action Buttons */}
        <div className="flex items-center justify-end gap-2.5 pt-1">
          {!isBusy && (
            <button
              type="button"
              onClick={onClose}
              className="px-3.5 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-300 transition-colors cursor-pointer"
            >
              Later
            </button>
          )}

          {updateInfo.releaseUrl && (
            <button
              type="button"
              onClick={() => window.open(updateInfo.releaseUrl, '_blank')}
              className="flex items-center gap-1.5 px-3 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-300 transition-colors cursor-pointer"
              title="View on GitHub"
            >
              <ExternalLink className="w-3.5 h-3.5" />
              <span>Release Notes</span>
            </button>
          )}

          <button
            type="button"
            disabled={isBusy}
            onClick={() => onInstall(updateInfo.assetUrl, updateInfo.releaseUrl)}
            className="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-xs font-semibold text-white shadow-lg shadow-indigo-500/20 transition-all cursor-pointer"
          >
            {isBusy ? (
              <>
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
                <span>Updating...</span>
              </>
            ) : (
              <>
                <Download className="w-3.5 h-3.5" />
                <span>Install Update</span>
              </>
            )}
          </button>
        </div>

      </div>
    </div>
  );
};
