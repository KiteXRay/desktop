import React, { useState } from 'react';
import { ShieldAlert, Copy, Check, Terminal, RefreshCw, X, KeyRound, Loader2 } from 'lucide-react';

interface PrivilegeModalProps {
  isOpen: boolean;
  command: string;
  errorMessage?: string;
  onClose: () => void;
  onCheckAgain: () => Promise<boolean>;
  onGrantWithPkexec: () => Promise<boolean>;
}

export const PrivilegeModal: React.FC<PrivilegeModalProps> = ({
  isOpen,
  command,
  errorMessage,
  onClose,
  onCheckAgain,
  onGrantWithPkexec,
}) => {
  const [copied, setCopied] = useState(false);
  const [isGranting, setIsGranting] = useState(false);
  const [isChecking, setIsChecking] = useState(false);
  const [grantError, setGrantError] = useState<string | null>(null);

  if (!isOpen) return null;

  const handleCopy = () => {
    navigator.clipboard.writeText(command);
    setCopied(true);
    setTimeout(() => setCopied(false), 2500);
  };

  const handleGrant = async () => {
    setIsGranting(true);
    setGrantError(null);
    try {
      const ok = await onGrantWithPkexec();
      if (ok) {
        onClose();
      } else {
        setGrantError('Failed to grant privileges. Please run the terminal command manually.');
      }
    } catch (err: any) {
      setGrantError(err?.message || 'Authentication cancelled or failed.');
    } finally {
      setIsGranting(false);
    }
  };

  const handleCheck = async () => {
    setIsChecking(true);
    setGrantError(null);
    try {
      const ok = await onCheckAgain();
      if (ok) {
        onClose();
      } else {
        setGrantError('Network privileges still missing. Please make sure the command ran with sudo.');
      }
    } catch (err: any) {
      setGrantError(err?.message || 'Check failed.');
    } finally {
      setIsChecking(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-xs animate-in fade-in duration-150">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-lg shadow-2xl overflow-hidden flex flex-col gap-4 p-6 animate-in zoom-in-95 duration-200">
        
        {/* Header */}
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-center gap-3">
            <div className="w-11 h-11 rounded-xl bg-amber-500/10 border border-amber-500/20 text-amber-400 flex items-center justify-center shrink-0">
              <ShieldAlert className="w-6 h-6" />
            </div>
            <div>
              <h3 className="text-base font-bold text-slate-100">Network Privileges Required</h3>
              <p className="text-xs text-slate-400 mt-0.5">
                Linux capabilities missing on Kite executable
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors cursor-pointer"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Explanation */}
        <p className="text-xs text-slate-300 leading-relaxed">
          Kite requires network capabilities (<code className="bg-slate-800 px-1 py-0.5 rounded text-amber-300 font-mono text-[11px]">CAP_NET_ADMIN</code>, <code className="bg-slate-800 px-1 py-0.5 rounded text-amber-300 font-mono text-[11px]">CAP_NET_RAW</code>, <code className="bg-slate-800 px-1 py-0.5 rounded text-amber-300 font-mono text-[11px]">CAP_NET_BIND_SERVICE</code>) to create virtual TUN adapters (<code className="text-indigo-300">kite0</code>) and configure routing tables without needing to run the whole app as root.
        </p>

        {errorMessage && (
          <div className="text-[11px] text-amber-400/90 bg-amber-950/30 border border-amber-500/20 px-3 py-2 rounded-xl">
            {errorMessage}
          </div>
        )}

        {/* Command Box */}
        <div className="flex flex-col gap-1.5">
          <div className="flex items-center justify-between text-xs text-slate-400 font-medium">
            <span className="flex items-center gap-1.5">
              <Terminal className="w-3.5 h-3.5 text-slate-400" />
              <span>Run in terminal:</span>
            </span>
            <button
              type="button"
              onClick={handleCopy}
              className="flex items-center gap-1 text-xs text-indigo-400 hover:text-indigo-300 transition-colors cursor-pointer"
            >
              {copied ? (
                <>
                  <Check className="w-3.5 h-3.5 text-emerald-400" />
                  <span className="text-emerald-400">Copied!</span>
                </>
              ) : (
                <>
                  <Copy className="w-3.5 h-3.5" />
                  <span>Copy Command</span>
                </>
              )}
            </button>
          </div>

          <div className="p-3 bg-slate-950/80 border border-slate-800 rounded-xl font-mono text-xs text-amber-200/90 break-all select-all leading-relaxed">
            {command || 'sudo setcap cap_net_raw,cap_net_admin,cap_net_bind_service+eip /opt/kite/kite'}
          </div>
        </div>

        {grantError && (
          <div className="text-xs text-rose-400 bg-rose-950/30 border border-rose-500/20 p-2.5 rounded-xl">
            {grantError}
          </div>
        )}

        {/* Action Buttons */}
        <div className="flex flex-wrap items-center justify-end gap-2.5 pt-2 border-t border-slate-800/80">
          <button
            type="button"
            onClick={onClose}
            className="px-3.5 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-300 transition-colors cursor-pointer"
          >
            Dismiss
          </button>

          <button
            type="button"
            disabled={isChecking || isGranting}
            onClick={handleCheck}
            className="flex items-center gap-1.5 px-3.5 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 disabled:opacity-50 text-xs font-medium text-slate-200 border border-slate-700 transition-colors cursor-pointer"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isChecking ? 'animate-spin' : ''}`} />
            <span>Check Again</span>
          </button>

          <button
            type="button"
            disabled={isGranting || isChecking}
            onClick={handleGrant}
            className="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 disabled:opacity-50 text-xs font-semibold text-white shadow-lg shadow-amber-600/20 transition-all cursor-pointer"
          >
            {isGranting ? (
              <>
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
                <span>Prompting pkexec...</span>
              </>
            ) : (
              <>
                <KeyRound className="w-3.5 h-3.5" />
                <span>Grant with Sudo (Prompt)</span>
              </>
            )}
          </button>
        </div>

      </div>
    </div>
  );
};
