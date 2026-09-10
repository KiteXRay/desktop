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
        <span
          className={`inline-flex items-center justify-center px-1.5 py-0.5 text-[9px] font-normal uppercase tracking-wider rounded border leading-none ${
            protocol.toLowerCase() === 'wireguard'
              ? 'bg-emerald-500/15 text-emerald-300 border-emerald-500/25'
              : protocol.toLowerCase() === 'awg'
              ? 'bg-teal-500/15 text-teal-300 border-teal-500/25'
              : 'bg-indigo-500/15 text-indigo-300 border-indigo-500/25'
          }`}
        >
          {protocol}
        </span>
      )}

      {security && security.toLowerCase() === 'reality' && (
        <span className="inline-flex items-center justify-center px-1.5 py-0.5 text-[9px] font-normal uppercase rounded border border-emerald-500/25 bg-emerald-500/15 text-emerald-300 leading-none">
          REALITY
        </span>
      )}

      {tls && tls.toLowerCase() === 'tls' && (
        <span className="inline-flex items-center justify-center px-1.5 py-0.5 text-[9px] font-normal uppercase rounded border border-cyan-500/25 bg-cyan-500/15 text-cyan-300 leading-none">
          TLS
        </span>
      )}

      {tls && tls.toLowerCase() === 'none' && !security && (
        <span className="inline-flex items-center justify-center px-1.5 py-0.5 text-[9px] font-normal uppercase rounded border border-rose-500/30 bg-rose-500/20 text-rose-300 leading-none">
          No TLS
        </span>
      )}

      {network && (
        <span className="inline-flex items-center justify-center px-1.5 py-0.5 text-[9px] font-normal uppercase rounded border border-slate-700/60 bg-slate-800 text-slate-300 leading-none">
          {network}
        </span>
      )}

      {flow && (
        <span className="inline-flex items-center justify-center px-1.5 py-0.5 text-[9px] font-normal font-mono rounded border border-violet-500/20 bg-violet-500/10 text-violet-300 leading-none">
          {flow}
        </span>
      )}
    </div>
  );
};
