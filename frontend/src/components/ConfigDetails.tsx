import React, { useState } from 'react';
import { Copy, Check, Rocket, Loader2 } from 'lucide-react';
import type { ConnectionDTO, TunnelMode } from '../types';

interface ConfigDetailsProps {
  connection: ConnectionDTO;
  tunnelMode?: TunnelMode;
  onSelectExecutable?: () => void;
  isLaunchingApp?: boolean;
}

export const ConfigDetails: React.FC<ConfigDetailsProps> = React.memo(({
  connection,
  tunnelMode = 'system',
  onSelectExecutable,
  isLaunchingApp = false,
}) => {
  const [copiedLink, setCopiedLink] = useState(false);
  const [copiedDetails, setCopiedDetails] = useState(false);

  const cfg = connection.configMap || {};

  const handleCopyLink = () => {
    navigator.clipboard.writeText(connection.link);
    setCopiedLink(true);
    setTimeout(() => setCopiedLink(false), 2000);
  };

  const handleCopyDetails = () => {
    const lines = Object.entries(cfg)
      .filter(([_, v]) => Boolean(v))
      .map(([k, v]) => `${k}: ${v}`)
      .join('\n');
    navigator.clipboard.writeText(lines);
    setCopiedDetails(true);
    setTimeout(() => setCopiedDetails(false), 2000);
  };

  // Important fields to display prominently
  const displayFields = [
    { key: 'Address', label: 'Server Host' },
    { key: 'Port', label: 'Port' },
    { key: 'Protocol', label: 'Protocol' },
    { key: 'Security', label: 'Security' },
    { key: 'TLS', label: 'TLS' },
    { key: 'SNI', label: 'SNI' },
    { key: 'ALPN', label: 'ALPN' },
    { key: 'TlsFingerprint', label: 'Fingerprint' },
    { key: 'Flow', label: 'Flow' },
    { key: 'Network', label: 'Transport' },
    { key: 'ServiceName', label: 'Service Name' },
    { key: 'Path', label: 'Path' },
    { key: 'Authority', label: 'Authority' },
    { key: 'ID', label: 'UUID / ID' },
  ];

  const presentFields = displayFields.filter(f => cfg[f.key]);
  const otherFields = Object.entries(cfg).filter(
    ([k, v]) => Boolean(v) && !displayFields.some(f => f.key === k)
  );

  return (
    <div className="flex flex-col gap-4">
      {/* Per-App Action Banner */}
      <div className="bg-gradient-to-r from-indigo-950/40 via-slate-900/60 to-purple-950/40 rounded-xl p-4 border border-indigo-500/20 flex flex-col gap-3 shadow-md">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-indigo-600/20 border border-indigo-500/30 flex items-center justify-center text-indigo-400 shrink-0">
              <Rocket className="w-4 h-4" />
            </div>
            <div>
              <h4 className="text-xs font-bold text-slate-100 flex items-center gap-2">
                App Tunnel
                <span className="text-[10px] font-mono text-indigo-400 bg-indigo-500/10 px-1.5 py-0.5 rounded border border-indigo-500/20">
                  {tunnelMode === 'per_app' ? 'Active Mode' : 'Selective'}
                </span>
              </h4>
              <p className="text-[11px] text-slate-400 mt-0.5">
                Select an executable to start the proxy and route only that app through &quot;{connection.label}&quot;.
              </p>
            </div>
          </div>
        </div>

        <button
          onClick={onSelectExecutable}
          disabled={isLaunchingApp}
          className="flex items-center justify-center gap-2 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-xs font-semibold rounded-xl shadow-md shadow-indigo-600/20 active:scale-95 transition-all cursor-pointer"
        >
          {isLaunchingApp ? (
            <>
              <Loader2 className="w-3.5 h-3.5 animate-spin text-indigo-200" />
              <span>Starting & Routing App...</span>
            </>
          ) : (
            <>
              <Rocket className="w-3.5 h-3.5" />
              <span>Select App</span>
            </>
          )}
        </button>
      </div>

      {/* Configuration Parameters Table */}
      <div className="bg-slate-900/40 rounded-xl p-4 border border-slate-800/80 flex flex-col gap-3">
      <div className="flex items-center justify-between border-b border-slate-800/80 pb-2.5">
        <h4 className="text-xs font-semibold uppercase tracking-wider text-slate-400">
          Configuration Parameters
        </h4>
        <div className="flex items-center gap-2">
          <button
            onClick={handleCopyLink}
            className="flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 transition-colors"
            title="Copy URL"
          >
            {copiedLink ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            <span>{copiedLink ? 'Copied URL' : 'Copy Link'}</span>
          </button>
          <button
            onClick={handleCopyDetails}
            className="flex items-center gap-1.5 px-2.5 py-1 text-xs rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 transition-colors"
            title="Copy Config Info"
          >
            {copiedDetails ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            <span>{copiedDetails ? 'Copied' : 'Copy All'}</span>
          </button>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-x-4 gap-y-2 text-xs selectable-text">
        {presentFields.map(f => (
          <div key={f.key} className="flex flex-col bg-slate-950/40 rounded-lg px-2.5 py-1.5 border border-slate-800/40">
            <span className="text-[10px] uppercase font-semibold text-slate-500 tracking-wider">
              {f.label}
            </span>
            <span className="font-mono text-slate-200 truncate select-all" title={cfg[f.key]}>
              {cfg[f.key]}
            </span>
          </div>
        ))}

        {otherFields.map(([k, v]) => (
          <div key={k} className="flex flex-col bg-slate-950/40 rounded-lg px-2.5 py-1.5 border border-slate-800/40">
            <span className="text-[10px] uppercase font-semibold text-slate-500 tracking-wider">
              {k}
            </span>
            <span className="font-mono text-slate-300 truncate select-all" title={v}>
              {v}
            </span>
          </div>
        ))}
      </div>
    </div>
  </div>
);
});
