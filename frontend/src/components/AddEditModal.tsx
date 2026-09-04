import React, { useState, useEffect, useRef } from 'react';
import {
  X,
  Clipboard,
  ShieldCheck,
  AlertCircle,
  Loader2,
  Sliders,
  Code,
  Server,
  Lock,
  Wifi,
  Shield,
  Eye,
  EyeOff,
  Check,
  ChevronDown,
} from 'lucide-react';
import { api } from '../api/wails';
import type { ConnectionDTO } from '../types';

interface AddEditModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  editItem?: ConnectionDTO | null;
}

interface StyledSelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
  label?: string;
  children: React.ReactNode;
}

const StyledSelect: React.FC<StyledSelectProps> = ({ label, className = '', children, ...props }) => (
  <div className="flex flex-col">
    {label && (
      <label className="block text-[11px] font-medium text-slate-400 mb-1">
        {label}
      </label>
    )}
    <div className="relative flex items-center">
      <select
        className={`w-full appearance-none px-3 py-1.5 pr-8 rounded-lg bg-slate-900 hover:bg-slate-900/90 border border-slate-800 hover:border-slate-700 focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500/40 text-slate-200 text-xs transition-all outline-hidden font-mono cursor-pointer shadow-xs ${className}`}
        {...props}
      >
        {children}
      </select>
      <ChevronDown className="absolute right-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-slate-400 pointer-events-none transition-colors" />
    </div>
  </div>
);

export const AddEditModal: React.FC<AddEditModalProps> = ({
  isOpen,
  onClose,
  onSuccess,
  editItem,
}) => {
  const [activeTab, setActiveTab] = useState<'form' | 'raw'>('form');
  const [label, setLabel] = useState('');
  const [link, setLink] = useState('');
  const [params, setParams] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [showSecret, setShowSecret] = useState(false);
  const [copiedLink, setCopiedLink] = useState(false);

  // Prevent link-building loops
  const isUpdatingFromLinkRef = useRef(false);

  useEffect(() => {
    if (!isOpen) return;

    if (editItem) {
      setActiveTab('form');
      setLabel(editItem.label);
      setLink(editItem.link);

      const initialMap: Record<string, string> = {
        Protocol: editItem.protocol || 'vless',
        Address: editItem.address || '',
        Port: editItem.port || '',
        Remark: editItem.label || '',
        Security: editItem.security || editItem.tls || 'none',
        Network: editItem.network || 'tcp',
        Flow: editItem.flow || '',
        ...(editItem.configMap || {}),
      };
      setParams(initialMap);

      // Also parse latest from link
      api.parseLinkPreview(editItem.link)
        .then(parsed => {
          setParams(prev => ({ ...prev, ...parsed }));
        })
        .catch(() => {});
    } else {
      setActiveTab('form');
      setLabel('');
      setLink('');
      setParams({
        Protocol: 'vless',
        Address: '',
        Port: '443',
        Network: 'tcp',
        Type: 'tcp',
        Security: 'none',
        TLS: 'none',
      });
    }
    setError(null);
  }, [editItem, isOpen]);

  // Live link validation & parser when in raw tab
  useEffect(() => {
    if (!isOpen || isUpdatingFromLinkRef.current) return;
    if (!link.trim()) {
      return;
    }

    const timer = setTimeout(async () => {
      try {
        const parsed = await api.parseLinkPreview(link.trim());
        setParams(prev => ({ ...prev, ...parsed }));
        setError(null);
        if (!label && parsed.Remark) {
          setLabel(parsed.Remark);
        }
      } catch (err: any) {
        // Link might be incomplete while typing
      }
    }, 250);

    return () => clearTimeout(timer);
  }, [link, isOpen]);

  if (!isOpen) return null;

  const updateParams = (updates: Record<string, string>) => {
    if (updates.Remark !== undefined) {
      setLabel(updates.Remark);
    }
    setParams(prev => {
      const next = { ...prev, ...updates };

      // Auto rebuild link
      api.buildLinkFromConfig(next)
        .then(generated => {
          if (generated) {
            isUpdatingFromLinkRef.current = true;
            setLink(generated);
            setTimeout(() => {
              isUpdatingFromLinkRef.current = false;
            }, 50);
          }
        })
        .catch(() => {
          // Incomplete fields while user is typing
        });

      return next;
    });
  };

  const updateParam = (key: string, val: string) => {
    updateParams({ [key]: val });
  };

  const handlePaste = async () => {
    try {
      const text = await navigator.clipboard.readText();
      if (text) {
        setLink(text.trim());
        const parsed = await api.parseLinkPreview(text.trim());
        setParams(parsed);
        if (parsed.Remark) {
          setLabel(parsed.Remark);
        }
        setError(null);
      }
    } catch {
      // Clipboard access blocked
    }
  };

  const handleCopyLink = () => {
    if (!link) return;
    navigator.clipboard.writeText(link);
    setCopiedLink(true);
    setTimeout(() => setCopiedLink(false), 2000);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);

    try {
      let finalLink = link.trim();

      // If submitted from form tab, re-generate canonical link from form parameters
      if (activeTab === 'form') {
        const payload = {
          ...params,
          Remark: label.trim() || params.Remark || '',
        };
        const built = await api.buildLinkFromConfig(payload);
        if (built) {
          finalLink = built;
        }
      }

      const finalLabel = label.trim() || params.Remark || params.Address || 'Profile';
      if (!finalLink) {
        throw new Error('Connection URL could not be generated. Check required fields (Protocol, Server, Port, ID).');
      }

      if (editItem) {
        await api.updateConnection(editItem.id, finalLabel, finalLink);
      } else {
        await api.addConnection(finalLabel, finalLink);
      }

      onSuccess();
      onClose();
    } catch (err: any) {
      setError(err?.message || String(err) || 'Failed to save connection');
    } finally {
      setLoading(false);
    }
  };

  const secVal = (params.Security || params.TLS || 'none').toLowerCase();
  const netVal = (params.Network || params.Type || 'tcp').toLowerCase();
  const protoVal = (params.Protocol || 'vless').toLowerCase();

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-xs p-4 animate-in fade-in duration-150">
      <div
        className="w-full max-w-2xl bg-slate-900 rounded-2xl border border-slate-800 shadow-2xl overflow-hidden flex flex-col max-h-[90vh]"
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-900/90 shrink-0">
          <div className="flex items-center gap-3 min-w-0 mr-2">
            <div className="w-10 h-10 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400 shrink-0">
              <Sliders className="w-5 h-5" />
            </div>
            <div className="min-w-0">
              <h3 className="text-sm font-bold text-slate-100 flex items-center gap-2 truncate">
                <span>{editItem ? 'Edit VPN Profile' : 'Add VPN Profile'}</span>
                {editItem && (
                  <span className="px-2 py-0.5 text-[10px] uppercase font-mono font-semibold bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 rounded-full shrink-0">
                    {protoVal}
                  </span>
                )}
              </h3>
              <p className="text-xs text-slate-400 mt-0.5 truncate">
                {activeTab === 'form'
                  ? 'Edit individual protocol parameters and connection settings'
                  : 'Import, inspect, or paste raw connection link URL'}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2 shrink-0">
            {/* Tab switch */}
            <div className="flex items-center bg-slate-950 p-1 rounded-xl border border-slate-800 shrink-0">
              <button
                type="button"
                onClick={() => setActiveTab('form')}
                className={`flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-medium whitespace-nowrap transition-all cursor-pointer ${
                  activeTab === 'form'
                    ? 'bg-indigo-600 text-white shadow-xs'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
                title="Edit each parsed parameter separately"
              >
                <Sliders className="w-3.5 h-3.5" />
                <span>Form</span>
              </button>
              <button
                type="button"
                onClick={() => setActiveTab('raw')}
                className={`flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-medium whitespace-nowrap transition-all cursor-pointer ${
                  activeTab === 'raw'
                    ? 'bg-indigo-600 text-white shadow-xs'
                    : 'text-slate-400 hover:text-slate-200'
                }`}
                title="View / edit raw connection link"
              >
                <Code className="w-3.5 h-3.5" />
                <span>Raw URL</span>
              </button>
            </div>

            <button
              onClick={onClose}
              className="p-1.5 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors cursor-pointer"
            >
              <X className="w-5 h-5" />
            </button>
          </div>
        </div>

        {/* Scrollable Form Body */}
        <form onSubmit={handleSubmit} className="flex-1 overflow-y-auto p-6 flex flex-col gap-5">
          {error && (
            <div className="flex items-start gap-2.5 p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-400 text-xs">
              <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
              <span>{error}</span>
            </div>
          )}

          {activeTab === 'form' ? (
            <div className="flex flex-col gap-5">
              {/* Section 1: Server & Protocol */}
              <div className="bg-slate-950/40 rounded-xl p-4 border border-slate-800/80 flex flex-col gap-3.5">
                <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-indigo-400">
                  <Server className="w-3.5 h-3.5" />
                  <span>General & Server Endpoint</span>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div className="sm:col-span-2">
                    <label className="block text-[11px] font-medium text-slate-400 mb-1">
                      Profile Display Name
                    </label>
                    <input
                      type="text"
                      value={label}
                      onChange={e => {
                        setLabel(e.target.value);
                        updateParam('Remark', e.target.value);
                      }}
                      placeholder="e.g. Frankfurt Fast, Home Server"
                      className="w-full px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-medium"
                    />
                  </div>

                  <StyledSelect
                    label="Protocol"
                    value={protoVal}
                    onChange={e => updateParam('Protocol', e.target.value)}
                  >
                    <option value="vless" className="bg-slate-900 text-slate-200 py-1">VLESS</option>
                    <option value="vmess" className="bg-slate-900 text-slate-200 py-1">VMess</option>
                    <option value="trojan" className="bg-slate-900 text-slate-200 py-1">Trojan</option>
                    <option value="ss" className="bg-slate-900 text-slate-200 py-1">Shadowsocks (SS)</option>
                  </StyledSelect>

                  <div>
                    <label className="block text-[11px] font-medium text-slate-400 mb-1">
                      Port
                    </label>
                    <input
                      type="text"
                      value={params.Port || ''}
                      onChange={e => updateParam('Port', e.target.value)}
                      placeholder="443"
                      className="w-full px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                    />
                  </div>

                  <div className="sm:col-span-2">
                    <label className="block text-[11px] font-medium text-slate-400 mb-1">
                      Server Address (Domain or IP)
                    </label>
                    <input
                      type="text"
                      value={params.Address || ''}
                      onChange={e => updateParam('Address', e.target.value)}
                      placeholder="e.g. vpn.example.com or 192.168.1.1"
                      className="w-full px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                    />
                  </div>
                </div>
              </div>

              {/* Section 2: Authentication & Security */}
              <div className="bg-slate-950/40 rounded-xl p-4 border border-slate-800/80 flex flex-col gap-3.5">
                <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-indigo-400">
                  <Lock className="w-3.5 h-3.5" />
                  <span>Authentication & Security</span>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <div className="sm:col-span-2">
                    <div className="flex items-center justify-between mb-1">
                      <label className="text-[11px] font-medium text-slate-400">
                        {protoVal === 'trojan' || protoVal === 'ss'
                          ? 'Password / Secret Key'
                          : 'UUID / User ID'}
                      </label>
                      <button
                        type="button"
                        onClick={() => setShowSecret(!showSecret)}
                        className="text-[11px] text-slate-500 hover:text-slate-300 flex items-center gap-1 cursor-pointer"
                      >
                        {showSecret ? <EyeOff className="w-3 h-3" /> : <Eye className="w-3 h-3" />}
                        <span>{showSecret ? 'Hide' : 'Show'}</span>
                      </button>
                    </div>
                    <input
                      type={showSecret ? 'text' : 'password'}
                      value={params.ID || ''}
                      onChange={e => updateParam('ID', e.target.value)}
                      placeholder="UUID or password..."
                      className="w-full px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                    />
                  </div>

                  <StyledSelect
                    label="Security / TLS"
                    value={secVal}
                    onChange={e => updateParams({ Security: e.target.value, TLS: e.target.value })}
                  >
                    <option value="none" className="bg-slate-900 text-slate-200 py-1">None (Plain)</option>
                    <option value="tls" className="bg-slate-900 text-slate-200 py-1">TLS</option>
                    <option value="reality" className="bg-slate-900 text-slate-200 py-1">REALITY</option>
                  </StyledSelect>

                  <div>
                    <label className="block text-[11px] font-medium text-slate-400 mb-1">
                      Flow
                    </label>
                    <input
                      type="text"
                      value={params.Flow || ''}
                      onChange={e => updateParam('Flow', e.target.value)}
                      placeholder="e.g. xtls-rprx-vision"
                      className="w-full px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                    />
                  </div>

                  {protoVal === 'vmess' && (
                    <div>
                      <label className="block text-[11px] font-medium text-slate-400 mb-1">
                        AlterID (aid)
                      </label>
                      <input
                        type="text"
                        value={params.Aid ?? '0'}
                        onChange={e => updateParam('Aid', e.target.value)}
                        placeholder="0"
                        className="w-full px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                      />
                    </div>
                  )}

                  {(protoVal === 'ss' || protoVal === 'vless' || protoVal === 'vmess') && (
                    <div>
                      <label className="block text-[11px] font-medium text-slate-400 mb-1">
                        Encryption / Cipher
                      </label>
                      <input
                        type="text"
                        value={params.Encryption || ''}
                        onChange={e => updateParam('Encryption', e.target.value)}
                        placeholder={protoVal === 'ss' ? 'aes-256-gcm' : 'none'}
                        className="w-full px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                      />
                    </div>
                  )}
                </div>
              </div>

              {/* Section 3: REALITY Settings (Only when security is reality) */}
              {secVal === 'reality' && (
                <div className="bg-gradient-to-br from-indigo-950/30 to-purple-950/30 rounded-xl p-4 border border-indigo-500/30 flex flex-col gap-3.5 animate-in fade-in">
                  <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-indigo-300">
                    <Shield className="w-3.5 h-3.5 text-indigo-400" />
                    <span>REALITY Parameters</span>
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                    <div className="sm:col-span-2">
                      <label className="block text-[11px] font-medium text-slate-400 mb-1">
                        Public Key (pbk)
                      </label>
                      <input
                        type="text"
                        value={params.Pbk || ''}
                        onChange={e => updateParam('Pbk', e.target.value)}
                        placeholder="Public key string..."
                        className="w-full px-3 py-1.5 rounded-lg bg-slate-900/90 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                      />
                    </div>

                    <div>
                      <label className="block text-[11px] font-medium text-slate-400 mb-1">
                        Short ID (sid)
                      </label>
                      <input
                        type="text"
                        value={params.Sid || ''}
                        onChange={e => updateParam('Sid', e.target.value)}
                        placeholder="e.g. 0c or empty"
                        className="w-full px-3 py-1.5 rounded-lg bg-slate-900/90 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                      />
                    </div>

                    <div>
                      <label className="block text-[11px] font-medium text-slate-400 mb-1">
                        SpiderX Path (spx)
                      </label>
                      <input
                        type="text"
                        value={params.Spx || ''}
                        onChange={e => updateParam('Spx', e.target.value)}
                        placeholder="e.g. /"
                        className="w-full px-3 py-1.5 rounded-lg bg-slate-900/90 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                      />
                    </div>
                  </div>
                </div>
              )}

              {/* Section 4: TLS Settings */}
              {(secVal === 'tls' || secVal === 'reality') && (
                <div className="bg-slate-950/40 rounded-xl p-4 border border-slate-800/80 flex flex-col gap-3.5">
                  <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-indigo-400">
                    <ShieldCheck className="w-3.5 h-3.5" />
                    <span>TLS & Handshake Settings</span>
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                    <div className="sm:col-span-2">
                      <label className="block text-[11px] font-medium text-slate-400 mb-1">
                        SNI (Server Name Indication)
                      </label>
                      <input
                        type="text"
                        value={params.SNI || ''}
                        onChange={e => updateParam('SNI', e.target.value)}
                        placeholder="e.g. yahoo.com, cloudflare.com"
                        className="w-full px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                      />
                    </div>

                    <StyledSelect
                      label="Fingerprint (fp)"
                      value={params.TlsFingerprint || params.Fp || 'chrome'}
                      onChange={e => updateParams({ TlsFingerprint: e.target.value, Fp: e.target.value })}
                    >
                      <option value="chrome" className="bg-slate-900 text-slate-200 py-1">chrome</option>
                      <option value="firefox" className="bg-slate-900 text-slate-200 py-1">firefox</option>
                      <option value="safari" className="bg-slate-900 text-slate-200 py-1">safari</option>
                      <option value="ios" className="bg-slate-900 text-slate-200 py-1">ios</option>
                      <option value="android" className="bg-slate-900 text-slate-200 py-1">android</option>
                      <option value="edge" className="bg-slate-900 text-slate-200 py-1">edge</option>
                      <option value="random" className="bg-slate-900 text-slate-200 py-1">random</option>
                      <option value="randomized" className="bg-slate-900 text-slate-200 py-1">randomized</option>
                    </StyledSelect>

                    <div className="sm:col-span-3">
                      <label className="block text-[11px] font-medium text-slate-400 mb-1">
                        ALPN
                      </label>
                      <input
                        type="text"
                        value={params.ALPN || ''}
                        onChange={e => updateParam('ALPN', e.target.value)}
                        placeholder="e.g. h2,http/1.1"
                        className="w-full px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                      />
                    </div>
                  </div>
                </div>
              )}

              {/* Section 5: Transport & Network */}
              <div className="bg-slate-950/40 rounded-xl p-4 border border-slate-800/80 flex flex-col gap-3.5">
                <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-indigo-400">
                  <Wifi className="w-3.5 h-3.5" />
                  <span>Transport & Network Stream</span>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  <StyledSelect
                    label="Transport / Network Type"
                    value={netVal}
                    onChange={e => updateParams({ Network: e.target.value, Type: e.target.value })}
                  >
                    <option value="tcp" className="bg-slate-900 text-slate-200 py-1">TCP</option>
                    <option value="ws" className="bg-slate-900 text-slate-200 py-1">WebSocket (ws)</option>
                    <option value="grpc" className="bg-slate-900 text-slate-200 py-1">gRPC</option>
                    <option value="h2" className="bg-slate-900 text-slate-200 py-1">HTTP/2 (h2)</option>
                    <option value="httpupgrade" className="bg-slate-900 text-slate-200 py-1">HTTPUpgrade</option>
                    <option value="splithttp" className="bg-slate-900 text-slate-200 py-1">SplitHTTP</option>
                    <option value="xhttp" className="bg-slate-900 text-slate-200 py-1">XHTTP (xhttp)</option>
                  </StyledSelect>

                  <div>
                    <label className="block text-[11px] font-medium text-slate-400 mb-1">
                      Header Type
                    </label>
                    <input
                      type="text"
                      value={params.HeaderType || ''}
                      onChange={e => updateParam('HeaderType', e.target.value)}
                      placeholder="e.g. none, http"
                      className="w-full px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                    />
                  </div>

                  <div>
                    <label className="block text-[11px] font-medium text-slate-400 mb-1">
                      Host
                    </label>
                    <input
                      type="text"
                      value={params.Host || ''}
                      onChange={e => updateParam('Host', e.target.value)}
                      placeholder="Host header..."
                      className="w-full px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                    />
                  </div>

                  <div>
                    <label className="block text-[11px] font-medium text-slate-400 mb-1">
                      Path
                    </label>
                    <input
                      type="text"
                      value={params.Path || ''}
                      onChange={e => updateParam('Path', e.target.value)}
                      placeholder="e.g. /ws or /"
                      className="w-full px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                    />
                  </div>

                  {(netVal === 'xhttp' || netVal === 'splithttp') && (
                    <div className="sm:col-span-2">
                      <label className="block text-[11px] font-medium text-slate-400 mb-1">
                        XHTTP Mode
                      </label>
                      <input
                        type="text"
                        value={params.Mode || ''}
                        onChange={e => updateParam('Mode', e.target.value)}
                        placeholder="e.g. auto, packet-up, stream-up"
                        className="w-full px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                      />
                    </div>
                  )}

                  {netVal === 'grpc' && (
                    <div className="sm:col-span-2">
                      <label className="block text-[11px] font-medium text-slate-400 mb-1">
                        gRPC Service Name
                      </label>
                      <input
                        type="text"
                        value={params.ServiceName || ''}
                        onChange={e => updateParam('ServiceName', e.target.value)}
                        placeholder="Service name..."
                        className="w-full px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 focus:border-indigo-500 text-slate-100 placeholder-slate-600 text-xs transition-colors outline-hidden font-mono"
                      />
                    </div>
                  )}
                </div>
              </div>

              {/* Dynamic Link Preview */}
              {link && (
                <div className="p-3 rounded-xl bg-slate-950/60 border border-slate-800 flex flex-col gap-1.5 text-xs">
                  <div className="flex items-center justify-between text-slate-400">
                    <span className="font-semibold text-[11px] uppercase tracking-wider text-indigo-400">
                      Generated Link Preview
                    </span>
                    <button
                      type="button"
                      onClick={handleCopyLink}
                      className="flex items-center gap-1 text-[11px] text-indigo-400 hover:text-indigo-300 cursor-pointer"
                    >
                      {copiedLink ? <Check className="w-3 h-3 text-emerald-400" /> : <Clipboard className="w-3 h-3" />}
                      <span>{copiedLink ? 'Copied' : 'Copy'}</span>
                    </button>
                  </div>
                  <div className="font-mono text-[10px] text-slate-400 break-all bg-slate-900/80 p-2 rounded-lg border border-slate-800/60">
                    {link}
                  </div>
                </div>
              )}
            </div>
          ) : (
            /* Raw URL Mode */
            <div className="flex flex-col gap-4">
              <div>
                <label className="block text-xs font-semibold uppercase tracking-wider text-slate-400 mb-1.5">
                  Display Name
                </label>
                <input
                  type="text"
                  value={label}
                  onChange={e => setLabel(e.target.value)}
                  placeholder="e.g. Frankfurt Fast, Home Wireguard, Tokyo Reality"
                  className="w-full px-3.5 py-2 rounded-xl bg-slate-950/70 border border-slate-800 focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 text-slate-100 placeholder-slate-500 text-sm transition-colors outline-hidden"
                />
              </div>

              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <label className="text-xs font-semibold uppercase tracking-wider text-slate-400">
                    Connection URL
                  </label>
                  <button
                    type="button"
                    onClick={handlePaste}
                    className="flex items-center gap-1 text-xs text-indigo-400 hover:text-indigo-300 font-medium cursor-pointer"
                  >
                    <Clipboard className="w-3.5 h-3.5" />
                    <span>Paste from clipboard</span>
                  </button>
                </div>
                <textarea
                  value={link}
                  onChange={e => setLink(e.target.value)}
                  rows={4}
                  placeholder="vless://..., vmess://..., trojan://..., ss://..."
                  className="w-full px-3.5 py-2 font-mono text-xs rounded-xl bg-slate-950/70 border border-slate-800 focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 text-slate-200 placeholder-slate-600 transition-colors outline-hidden resize-none"
                />
              </div>

              {/* Protocol Preview */}
              {params.Protocol && (
                <div className="p-4 rounded-xl bg-indigo-950/30 border border-indigo-500/20 text-xs">
                  <div className="flex items-center gap-1.5 text-indigo-400 font-semibold mb-2.5">
                    <ShieldCheck className="w-4 h-4" />
                    <span>Parsed Parameters Preview</span>
                  </div>
                  <div className="grid grid-cols-2 gap-2 text-slate-300 font-mono text-[11px]">
                    <div>
                      <span className="text-slate-500">Protocol:</span> {params.Protocol}
                    </div>
                    <div>
                      <span className="text-slate-500">Server:</span> {params.Address}:{params.Port}
                    </div>
                    <div>
                      <span className="text-slate-500">Security:</span>{' '}
                      {params.Security || params.TLS || 'None'}
                    </div>
                    <div>
                      <span className="text-slate-500">Transport:</span>{' '}
                      {params.Network || params.Type || 'TCP'}
                    </div>
                    {params.SNI && (
                      <div>
                        <span className="text-slate-500">SNI:</span> {params.SNI}
                      </div>
                    )}
                    {params.Flow && (
                      <div>
                        <span className="text-slate-500">Flow:</span> {params.Flow}
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Actions */}
          <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-800 mt-2 shrink-0">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-xs font-medium text-slate-300 hover:text-white rounded-xl hover:bg-slate-800 transition-colors cursor-pointer"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="flex items-center gap-2 px-5 py-2 text-xs font-semibold text-white bg-indigo-600 hover:bg-indigo-500 active:bg-indigo-700 disabled:opacity-50 rounded-xl transition-all shadow-lg shadow-indigo-600/20 cursor-pointer"
            >
              {loading && <Loader2 className="w-4 h-4 animate-spin" />}
              <span>{editItem ? 'Save Changes' : 'Add Profile'}</span>
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
