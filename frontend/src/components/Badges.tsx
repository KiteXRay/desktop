import React from 'react';

interface ProtocolBadgeProps {
  protocol?: string;
  tls?: string;
  flow?: string;
  network?: string;
  security?: string;
}

export const ProtocolBadges: React.FC<ProtocolBadgeProps> = ({
  protocol,
  tls,
  flow,
  network,
  security,
}) => {
  return (
    <div className="flex flex-wrap items-center gap-1">
      {protocol && (
        <span className="px-1.5 py-0.5 text-[9px] font-semibold uppercase tracking-wider rounded bg-indigo-500/15 text-indigo-300 border border-indigo-500/25">
          {protocol}
        </span>
      )}

      {security && security.toLowerCase() === 'reality' && (
        <span className="px-1.5 py-0.5 text-[9px] font-semibold uppercase rounded bg-emerald-500/15 text-emerald-300 border border-emerald-500/25">
          REALITY
        </span>
      )}

      {tls && tls.toLowerCase() === 'tls' && (
        <span className="px-1.5 py-0.5 text-[9px] font-semibold uppercase rounded bg-cyan-500/15 text-cyan-300 border border-cyan-500/25">
          TLS
        </span>
      )}

      {tls && tls.toLowerCase() === 'none' && !security && (
        <span className="px-1.5 py-0.5 text-[9px] font-semibold uppercase rounded bg-rose-500/20 text-rose-300 border border-rose-500/30">
          No TLS
        </span>
      )}

      {network && (
        <span className="px-1.5 py-0.5 text-[9px] font-medium uppercase rounded bg-slate-800 text-slate-300 border border-slate-700/60">
          {network}
        </span>
      )}

      {flow && (
        <span className="px-1.5 py-0.5 text-[9px] font-mono rounded bg-violet-500/10 text-violet-300 border border-violet-500/20">
          {flow}
        </span>
      )}
    </div>
  );
};
