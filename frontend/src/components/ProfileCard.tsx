import React from 'react';
import { Power, ArrowUp, ArrowDown, Edit2, Trash2, Loader2, RotateCcw, Activity } from 'lucide-react';
import type { ConnectionDTO } from '../types';
import { ProtocolBadges } from './Badges';
import { formatBytes } from '../utils/formatters';

interface ProfileCardProps {
  connection: ConnectionDTO;
  index: number;
  isSelected: boolean;
  onSelect: () => void;
  onConnect: () => void;
  onEdit: () => void;
  onDelete: () => void;
  onResetTraffic: () => void;
  onPing?: () => void;
  isPinging?: boolean;
  onDragStart: (e: React.DragEvent, index: number) => void;
  onDragOver: (e: React.DragEvent, index: number) => void;
  onDrop: (e: React.DragEvent, index: number) => void;
  onDragEnd: (e: React.DragEvent) => void;
  isDragging?: boolean;
  isDragOver?: boolean;
  isConnecting?: boolean;
  isDisconnecting?: boolean;
}

export const ProfileCard: React.FC<ProfileCardProps> = React.memo(({
  connection,
  index,
  isSelected,
  onSelect,
  onConnect,
  onEdit,
  onDelete,
  onResetTraffic,
  onPing,
  isPinging,
  onDragStart,
  onDragOver,
  onDrop,
  onDragEnd,
  isDragging,
  isDragOver,
  isConnecting,
  isDisconnecting,
}) => {
  const { active, label, address, port, protocol, tls, flow, network, security, bytesRead, bytesWritten, totalBytes, pingMs } = connection;

  return (
    <div
      onClick={onSelect}
      draggable={!active}
      onDragStart={e => onDragStart(e, index)}
      onDragOver={e => onDragOver(e, index)}
      onDrop={e => onDrop(e, index)}
      onDragEnd={onDragEnd}
      className={`group relative rounded-xl p-2.5 transition-all duration-150 cursor-pointer border select-none ${
        isDragging
          ? 'opacity-35 border-dashed border-indigo-400 bg-slate-900/30'
          : isDragOver
          ? 'border-indigo-500 ring-2 ring-indigo-500/40 bg-indigo-950/20'
          : active
          ? 'bg-slate-900/90 border-emerald-500/60 shadow-md shadow-emerald-950/40 ring-1 ring-emerald-500/30'
          : isSelected
          ? 'bg-slate-900/70 border-indigo-500/50 shadow-xs ring-1 ring-indigo-500/20'
          : 'bg-slate-900/40 border-slate-800/80 hover:bg-slate-900/60 hover:border-slate-700/80'
      }`}
    >
      {/* Row 1: Status Dot, Title, Ping & Connect Action */}
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 min-w-0 flex-1">
          {/* Status Indicator Dot */}
          <div className="shrink-0 flex items-center justify-center">
            {isConnecting ? (
              <Loader2 className="w-3.5 h-3.5 text-amber-400 animate-spin" />
            ) : active ? (
              <div className="relative flex items-center justify-center w-3.5 h-3.5">
                <span className="relative w-2 h-2 rounded-full bg-emerald-400 shadow-[0_0_8px_#34d399]" />
              </div>
            ) : (
              <span className="w-2 h-2 rounded-full bg-slate-600" />
            )}
          </div>

          {/* Title + Status Badge */}
          <div className="flex items-center gap-2 min-w-0 flex-1">
            <h3 className="text-xs font-semibold text-slate-100 truncate" title={label}>
              {label}
            </h3>
            {isConnecting ? (
              <span className="px-1.5 py-0.2 text-[9px] font-medium rounded-full bg-amber-500/20 text-amber-300 border border-amber-500/30 shrink-0">
                Connecting
              </span>
            ) : isDisconnecting ? (
              <span className="px-1.5 py-0.2 text-[9px] font-medium rounded-full bg-rose-500/20 text-rose-300 border border-rose-500/30 shrink-0">
                Disconnecting
              </span>
            ) : active ? (
              <span className="px-1.5 py-0.2 text-[9px] font-medium rounded-full bg-emerald-500/20 text-emerald-400 border border-emerald-500/30 shrink-0">
                Active
              </span>
            ) : null}
          </div>
        </div>

        {/* Right actions: Ping & Connect button */}
        <div className="flex items-center gap-1.5 shrink-0" onClick={e => e.stopPropagation()}>
          <button
            onClick={onPing}
            disabled={isPinging}
            className={`flex items-center gap-1 px-1.5 py-0.5 rounded-md text-[10px] font-mono transition-colors border cursor-pointer ${
              isPinging
                ? 'text-indigo-300 bg-indigo-950/40 border-indigo-700/50'
                : pingMs !== undefined && pingMs > 0
                ? pingMs < 120
                  ? 'text-emerald-400 bg-emerald-950/30 border-emerald-800/40 hover:bg-emerald-900/40'
                  : pingMs < 250
                  ? 'text-amber-400 bg-amber-950/30 border-amber-800/40 hover:bg-amber-900/40'
                  : 'text-rose-400 bg-rose-950/30 border-rose-800/40 hover:bg-rose-900/40'
                : pingMs === -1
                ? 'text-rose-400 bg-rose-950/30 border-rose-800/40'
                : 'text-slate-500 bg-slate-800/40 border-slate-700/40 hover:text-slate-300 hover:border-slate-600/50'
            }`}
            title={isPinging ? 'Pinging...' : 'Click to test latency'}
          >
            <Activity className={`w-2.5 h-2.5 ${isPinging ? 'animate-spin' : ''}`} />
            <span>
              {isPinging
                ? '...'
                : pingMs !== undefined && pingMs > 0
                ? `${pingMs} ms`
                : pingMs === -1
                ? 'Timeout'
                : '- ms'}
            </span>
          </button>

          <button
            onClick={onConnect}
            disabled={isConnecting || isDisconnecting}
            className={`flex items-center gap-1 px-2.5 py-1 rounded-lg font-medium text-[11px] transition-all shadow-xs ${
              isConnecting
                ? 'bg-amber-500/20 text-amber-300 border border-amber-500/40 cursor-wait'
                : isDisconnecting
                ? 'bg-rose-500/20 text-rose-300 border border-rose-500/40 cursor-wait'
                : active
                ? 'bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 border border-rose-500/40 active:scale-95'
                : 'bg-emerald-600 hover:bg-emerald-500 text-white shadow-emerald-600/20 active:scale-95'
            }`}
            title={active ? 'Disconnect' : 'Connect'}
          >
            {isConnecting ? (
              <>
                <Loader2 className="w-3 h-3 animate-spin text-amber-400" />
                <span>Connecting...</span>
              </>
            ) : isDisconnecting ? (
              <>
                <Loader2 className="w-3 h-3 animate-spin text-rose-400" />
                <span>Disconnecting...</span>
              </>
            ) : active ? (
              <>
                <Power className="w-3 h-3" />
                <span>Disconnect</span>
              </>
            ) : (
              <>
                <Power className="w-3 h-3" />
                <span>Connect</span>
              </>
            )}
          </button>
        </div>
      </div>

      {/* Row 2: Address (left) & Traffic stats (right) */}
      <div className="flex items-center justify-between gap-2 pl-[22px] mt-1 text-[11px] font-mono text-slate-400">
        <div className="truncate text-slate-400" title={address ? `${address}:${port}` : 'Local profile'}>
          {address ? `${address}:${port}` : 'Local profile'}
        </div>

        <div className="flex items-center gap-2 shrink-0">
          <span className="flex items-center gap-0.5" title="Download">
            <ArrowDown className="w-3 h-3 text-sky-400" />
            <span>{formatBytes(bytesWritten)}</span>
          </span>
          <span className="flex items-center gap-0.5" title="Upload">
            <ArrowUp className="w-3 h-3 text-emerald-400" />
            <span>{formatBytes(bytesRead)}</span>
          </span>
          <span className="text-slate-600 font-sans">•</span>
          <span className="font-semibold text-slate-300" title="Total">
            {formatBytes(totalBytes ?? (bytesRead + bytesWritten))}
          </span>
        </div>
      </div>

      {/* Row 3: Protocol Tags (left) & Reset/Edit/Remove buttons (bottom right corner) */}
      <div className="flex items-center justify-between gap-2 pl-[22px] mt-1.5">
        <ProtocolBadges
          protocol={protocol}
          tls={tls}
          flow={flow}
          network={network}
          security={security}
        />

        <div className="flex items-center gap-0.5 shrink-0 opacity-60 group-hover:opacity-100 transition-opacity" onClick={e => e.stopPropagation()}>
          <button
            onClick={onResetTraffic}
            className="p-1 text-slate-400 hover:text-amber-400 rounded-md hover:bg-slate-800 transition-colors cursor-pointer"
            title="Reset profile traffic statistics"
          >
            <RotateCcw className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={onEdit}
            disabled={active}
            className="p-1 text-slate-400 hover:text-slate-200 disabled:opacity-30 disabled:hover:text-slate-400 rounded-md hover:bg-slate-800 transition-colors cursor-pointer"
            title={active ? 'Disconnect before editing' : 'Edit profile'}
          >
            <Edit2 className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={onDelete}
            disabled={active}
            className="p-1 text-slate-400 hover:text-rose-400 disabled:opacity-30 disabled:hover:text-slate-400 rounded-md hover:bg-slate-800 transition-colors cursor-pointer"
            title={active ? 'Disconnect before deleting' : 'Delete profile'}
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </div>
  );
});
