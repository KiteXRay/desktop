import React, { useState, useEffect } from 'react';
import {
  Plus,
  Pencil,
  Trash2,
  ToggleLeft,
  ToggleRight,
  AppWindow,
  Check,
  Layers,
  Copy,
  Terminal,
  X,
  ChevronDown,
  Folder,
  FolderPlus,
  AlertTriangle,
} from 'lucide-react';
import { api } from '../api/wails';
import type { BridgeGroup, BridgeRule, ProxyEndpointsDTO, InstalledApp } from '../types';
import { SelectAppModal } from './SelectAppModal';

interface BridgeViewProps {
  isConnected: boolean;
  activeLabel?: string;
  onConnect?: () => void;
}

export const BridgeView: React.FC<BridgeViewProps> = () => {
  const [groups, setGroups] = useState<BridgeGroup[]>([]);
  const [rules, setRules] = useState<BridgeRule[]>([]);
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});
  const [runningCounts, setRunningCounts] = useState<Record<string, number>>({});
  const [endpoints, setEndpoints] = useState<ProxyEndpointsDTO>({
    socks5Host: '127.0.0.1',
    socks5Port: 10808,
    httpHost: '127.0.0.1',
    httpPort: 10809,
    socks5Url: 'socks5://127.0.0.1:10808',
    httpUrl: 'http://127.0.0.1:10809',
  });

  // Modal states
  const [isRuleModalOpen, setIsRuleModalOpen] = useState(false);
  const [isGroupModalOpen, setIsGroupModalOpen] = useState(false);
  const [isSelectAppOpen, setIsSelectAppOpen] = useState(false);
  const [groupToDelete, setGroupToDelete] = useState<BridgeGroup | null>(null);

  // Editing states
  const [editingRule, setEditingRule] = useState<BridgeRule | null>(null);
  const [editingGroup, setEditingGroup] = useState<BridgeGroup | null>(null);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  // Rule Form state
  const [pattern, setPattern] = useState('');
  const [proxyType, setProxyType] = useState<'socks5' | 'http'>('socks5');
  const [customTarget, setCustomTarget] = useState('');
  const [description, setDescription] = useState('');
  const [ruleGroupId, setRuleGroupId] = useState('');

  // Group Form state
  const [groupName, setGroupName] = useState('');

  // Load initial groups, rules and proxy endpoints
  useEffect(() => {
    Promise.all([
      api.getBridgeGroups(),
      api.getBridgeRules(),
      api.getProxyEndpoints(),
    ]).then(([loadedGroups, loadedRules, loadedEndpoints]) => {
      setEndpoints(loadedEndpoints);

      let initialGroups = loadedGroups || [];
      if (initialGroups.length === 0) {
        initialGroups = [
          {
            id: 'group-default',
            name: 'Default',
            enabled: true,
          },
        ];
        api.saveBridgeGroups(initialGroups);
      }
      setGroups(initialGroups);

      // Normalize rules so that each rule has a valid groupId
      const firstGroupId = initialGroups[0].id;
      let rulesNeedUpdate = false;
      const normalizedRules = (loadedRules || []).map((r) => {
        if (!r.groupId || !initialGroups.some((g) => g.id === r.groupId)) {
          rulesNeedUpdate = true;
          return { ...r, groupId: firstGroupId };
        }
        return r;
      });

      if (rulesNeedUpdate) {
        api.saveBridgeRules(normalizedRules);
      }
      setRules(normalizedRules);
    });
  }, []);

  // Listen to remote changes if any
  useEffect(() => {
    const unsubGroups = api.onBridgeGroupsChanged((updated) => {
      if (updated && updated.length > 0) {
        setGroups(updated);
      }
    });
    const unsubRules = api.onBridgeRulesChanged((updated) => {
      if (updated) {
        setRules(updated);
      }
    });
    return () => {
      unsubGroups();
      unsubRules();
    };
  }, []);

  // Poll running process counts every 3 seconds
  useEffect(() => {
    const checkProcesses = () => {
      api.checkRunningBridgeProcesses()
        .then((counts) => {
          if (counts) setRunningCounts(counts);
        })
        .catch(() => {});
    };

    checkProcesses();
    const interval = setInterval(checkProcesses, 3000);
    return () => clearInterval(interval);
  }, [rules, groups]);

  const saveGroups = async (updated: BridgeGroup[]) => {
    setGroups(updated);
    await api.saveBridgeGroups(updated);
  };

  const saveRules = async (updated: BridgeRule[]) => {
    setRules(updated);
    await api.saveBridgeRules(updated);
  };

  // Group Handlers
  const toggleGroupCollapse = (groupId: string) => {
    setCollapsedGroups((prev) => ({
      ...prev,
      [groupId]: !prev[groupId],
    }));
  };

  const handleToggleGroup = async (groupId: string, e?: React.MouseEvent) => {
    e?.stopPropagation();
    const updated = groups.map((g) =>
      g.id === groupId ? { ...g, enabled: !g.enabled } : g
    );
    await saveGroups(updated);
  };

  const handleOpenAddGroupModal = () => {
    setEditingGroup(null);
    setGroupName('');
    setIsGroupModalOpen(true);
  };

  const handleOpenEditGroupModal = (group: BridgeGroup, e?: React.MouseEvent) => {
    e?.stopPropagation();
    setEditingGroup(group);
    setGroupName(group.name);
    setIsGroupModalOpen(true);
  };

  const handleSubmitGroup = async (e: React.FormEvent) => {
    e.preventDefault();
    const cleanName = groupName.trim();
    if (!cleanName) return;

    if (editingGroup) {
      const updated = groups.map((g) =>
        g.id === editingGroup.id ? { ...g, name: cleanName } : g
      );
      await saveGroups(updated);
    } else {
      const newGroup: BridgeGroup = {
        id: 'group-' + Date.now(),
        name: cleanName,
        enabled: true,
      };
      await saveGroups([...groups, newGroup]);
    }

    setIsGroupModalOpen(false);
    setEditingGroup(null);
    setGroupName('');
  };

  const handleConfirmDeleteGroup = async () => {
    if (!groupToDelete) return;
    const targetId = groupToDelete.id;

    // Filter out the group and all its rules
    const updatedGroups = groups.filter((g) => g.id !== targetId);
    const updatedRules = rules.filter((r) => r.groupId !== targetId);

    // If all groups were removed, create a fresh Default group
    if (updatedGroups.length === 0) {
      const defaultGroup: BridgeGroup = {
        id: 'group-default',
        name: 'Default',
        enabled: true,
      };
      updatedGroups.push(defaultGroup);
    }

    await saveGroups(updatedGroups);
    await saveRules(updatedRules);
    setGroupToDelete(null);
  };

  // Rule Handlers
  const handleToggleRule = async (id: string) => {
    const updated = rules.map((r) =>
      r.id === id ? { ...r, enabled: !r.enabled } : r
    );
    await saveRules(updated);
  };

  const handleDeleteRule = async (id: string) => {
    const updated = rules.filter((r) => r.id !== id);
    await saveRules(updated);
  };

  const handleOpenAddModal = (presetGroupId?: string) => {
    setEditingRule(null);
    setPattern('');
    setProxyType('socks5');
    setCustomTarget('');
    setDescription('');
    const targetGroup = presetGroupId || (groups.length > 0 ? groups[0].id : '');
    setRuleGroupId(targetGroup);
    setIsRuleModalOpen(true);
  };

  const handleOpenEditModal = (rule: BridgeRule) => {
    setEditingRule(rule);
    setPattern(rule.pattern);
    setProxyType(rule.proxyType);
    setRuleGroupId(rule.groupId || (groups.length > 0 ? groups[0].id : ''));
    const isStandardTarget =
      rule.proxyTarget === `socks5://127.0.0.1:${endpoints.socks5Port}` ||
      rule.proxyTarget === `http://127.0.0.1:${endpoints.httpPort}` ||
      rule.proxyTarget === 'socks5://127.0.0.1:10808' ||
      rule.proxyTarget === 'http://127.0.0.1:10809';
    setCustomTarget(isStandardTarget ? '' : rule.proxyTarget);
    setDescription(rule.description || '');
    setIsRuleModalOpen(true);
  };

  const handleSubmitRule = async (e: React.FormEvent) => {
    e.preventDefault();
    const cleanPattern = pattern.trim();
    if (!cleanPattern) return;

    let target = customTarget.trim();
    if (!target) {
      target =
        proxyType === 'http'
          ? `http://127.0.0.1:${endpoints.httpPort}`
          : `socks5://127.0.0.1:${endpoints.socks5Port}`;
    }

    const assignedGroupId = ruleGroupId || (groups.length > 0 ? groups[0].id : 'group-default');

    if (editingRule) {
      const updated = rules.map((r) =>
        r.id === editingRule.id
          ? {
              ...r,
              groupId: assignedGroupId,
              pattern: cleanPattern,
              proxyTarget: target,
              proxyType,
              description: description.trim() || undefined,
            }
          : r
      );
      await saveRules(updated);
    } else {
      const newRule: BridgeRule = {
        id: 'rule-' + Date.now(),
        groupId: assignedGroupId,
        pattern: cleanPattern,
        proxyTarget: target,
        proxyType,
        enabled: true,
        description: description.trim() || undefined,
      };
      await saveRules([...rules, newRule]);
    }

    setIsRuleModalOpen(false);
    setEditingRule(null);
  };

  const handleSelectAppForForm = (app: InstalledApp) => {
    const baseName = app.exePath.split(/[\\/]/).pop() || app.name;
    setPattern(baseName);
    if (!description) {
      setDescription(app.name);
    }
    setIsSelectAppOpen(false);
  };

  const handleBrowseManualForForm = async () => {
    try {
      const exe = await api.selectExecutableDialog();
      if (exe) {
        const baseName = exe.split(/[\\/]/).pop() || exe;
        setPattern(baseName);
        if (!description) {
          setDescription(exe);
        }
        setIsSelectAppOpen(false);
      }
    } catch (err: any) {
      console.error('File dialog error:', err);
    }
  };

  const copyToClipboard = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  return (
    <div className="space-y-6 pb-8 animate-in fade-in duration-200">
      {/* Top Banner / Controls */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 p-5 rounded-2xl bg-slate-900/60 border border-slate-800 backdrop-blur-xs">
        <div>
          <div className="flex items-center gap-2">
            <h2 className="text-base font-semibold text-slate-100 flex items-center gap-2">
              <Layers className="w-4 h-4 text-indigo-400" />
              Bridge Mode Rules
            </h2>
            <span className="px-2 py-0.5 rounded-full text-[10px] font-medium bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
              Regex & Wildcard
            </span>
          </div>
          <p className="text-xs text-slate-400 mt-0.5">
            Organize executable patterns and rules into custom collapsible groups.
          </p>
        </div>

        {/* Action Buttons */}
        <div className="flex items-center gap-2 shrink-0">
          <button
            type="button"
            onClick={handleOpenAddGroupModal}
            className="px-3.5 py-1.5 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium flex items-center gap-1.5 transition-colors shadow-xs cursor-pointer"
          >
            <FolderPlus className="w-3.5 h-3.5" />
            Add Group
          </button>
        </div>
      </div>

      {/* Groups Container */}
      <div className="space-y-4">
        {groups.length === 0 ? (
          <div className="p-12 text-center border border-slate-800 rounded-2xl bg-slate-900/30">
            <div className="w-12 h-12 rounded-2xl bg-slate-800/60 border border-slate-700/50 flex items-center justify-center text-slate-400 mx-auto mb-3">
              <Folder className="w-6 h-6 text-indigo-400" />
            </div>
            <h3 className="text-sm font-semibold text-slate-200">No Bridge Groups Configured</h3>
            <p className="text-xs text-slate-400 max-w-sm mx-auto mt-1 mb-4">
              Create a group to organize and manage rules for Bridge Mode routing.
            </p>
            <button
              type="button"
              onClick={handleOpenAddGroupModal}
              className="px-4 py-2 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium inline-flex items-center gap-1.5 transition-colors shadow-xs cursor-pointer"
            >
              <FolderPlus className="w-4 h-4" />
              <span>Create First Group</span>
            </button>
          </div>
        ) : (
          groups.map((group) => {
            const isCollapsed = !!collapsedGroups[group.id];
            const groupRules = rules.filter(
              (r) => r.groupId === group.id || (!r.groupId && group.id === groups[0].id)
            );

            // Group live status: calculate active running count across rules in this group
            const groupRunningCount = groupRules.reduce((acc, r) => {
              if (r.enabled && group.enabled) {
                return acc + (runningCounts[r.id] || 0);
              }
              return acc;
            }, 0);

            const isGroupRunning = group.enabled && groupRunningCount > 0;

            return (
              <div
                key={group.id}
                className="border border-slate-800 rounded-2xl bg-slate-900/40 overflow-hidden shadow-xs transition-colors"
              >
                {/* Group Header */}
                <div
                  onClick={() => toggleGroupCollapse(group.id)}
                  className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 px-4 py-3 bg-slate-900/80 border-b border-slate-800/80 cursor-pointer hover:bg-slate-900 select-none transition-colors"
                >
                  {/* Left: Collapse, Icon, Name, Count */}
                  <div className="flex items-center gap-2.5 min-w-0">
                    <button
                      type="button"
                      onClick={(e) => {
                        e.stopPropagation();
                        toggleGroupCollapse(group.id);
                      }}
                      className="p-1 text-slate-400 hover:text-slate-200 transition-colors cursor-pointer"
                      title={isCollapsed ? 'Expand group' : 'Collapse group'}
                    >
                      <ChevronDown
                        className={`w-4 h-4 text-slate-400 transition-transform duration-200 ${
                          isCollapsed ? '-rotate-90' : ''
                        }`}
                      />
                    </button>

                    <div className="w-7 h-7 rounded-lg bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center text-indigo-400 shrink-0">
                      <Folder className="w-4 h-4" />
                    </div>

                    <div className="flex items-center gap-2 min-w-0">
                      <span className="text-xs font-semibold text-slate-100 truncate">
                        {group.name}
                      </span>
                      <button
                        type="button"
                        onClick={(e) => handleOpenEditGroupModal(group, e)}
                        title="Rename Group"
                        className="p-1 rounded-md text-slate-500 hover:text-indigo-300 hover:bg-slate-800 transition-colors cursor-pointer"
                      >
                        <Pencil className="w-3 h-3" />
                      </button>
                    </div>

                    <span className="px-2 py-0.5 rounded-full text-[10px] font-mono font-medium bg-slate-800 text-slate-400 border border-slate-700/50 shrink-0">
                      {groupRules.length} {groupRules.length === 1 ? 'rule' : 'rules'}
                    </span>
                  </div>

                  {/* Right: Live Status, Enable/Disable, Quick Add, Remove */}
                  <div
                    className="flex items-center gap-3 shrink-0 ml-auto sm:ml-0"
                    onClick={(e) => e.stopPropagation()}
                  >
                    {/* Live Status badge */}
                    {isGroupRunning ? (
                      <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                        <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
                        Running ({groupRunningCount})
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-[10px] font-medium bg-slate-800 text-slate-400 border border-slate-700/50">
                        <span className="w-1.5 h-1.5 rounded-full bg-slate-500" />
                        {group.enabled ? 'Inactive' : 'Disabled'}
                      </span>
                    )}

                    {/* Enable / Disable switch */}
                    <div className="flex items-center gap-1.5 border-l border-slate-800 pl-3">
                      <button
                        type="button"
                        onClick={(e) => handleToggleGroup(group.id, e)}
                        className="text-slate-400 hover:text-slate-200 transition-colors cursor-pointer"
                        title={group.enabled ? 'Disable Group' : 'Enable Group'}
                      >
                        {group.enabled ? (
                          <ToggleRight className="w-6 h-6 text-indigo-500" />
                        ) : (
                          <ToggleLeft className="w-6 h-6 text-slate-600" />
                        )}
                      </button>
                    </div>

                    {/* Add Rule to this group */}
                    <button
                      type="button"
                      onClick={() => handleOpenAddModal(group.id)}
                      title={`Add rule to ${group.name}`}
                      className="px-2.5 py-1 rounded-lg text-[11px] font-medium text-slate-300 hover:text-white bg-slate-800 hover:bg-slate-700 border border-slate-700/60 transition-colors flex items-center gap-1 cursor-pointer"
                    >
                      <Plus className="w-3 h-3" />
                      <span>Add Rule</span>
                    </button>

                    {/* Remove Group Button */}
                    <button
                      type="button"
                      onClick={() => setGroupToDelete(group)}
                      title="Remove Group"
                      className="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>

                {/* Group Content (Collapsible) */}
                {!isCollapsed && (
                  <div className={`transition-opacity ${!group.enabled ? 'opacity-60' : ''}`}>
                    {groupRules.length === 0 ? (
                      <div className="py-8 px-4 text-center text-slate-500 text-xs bg-slate-950/20">
                        <p className="text-slate-400 font-medium">No rules in this group</p>
                        <p className="text-slate-500 mt-1">
                          Click "+ Rule" above to route applications through this group.
                        </p>
                      </div>
                    ) : (
                      <div className="overflow-x-auto">
                        <table className="w-full text-left border-collapse">
                          <thead>
                            <tr className="border-b border-slate-800/70 bg-slate-950/40 text-[10px] font-semibold text-slate-400 uppercase tracking-wider">
                              <th className="py-2.5 px-4">Process / Pattern</th>
                              <th className="py-2.5 px-4">Proxy Target</th>
                              <th className="py-2.5 px-4">Live Status</th>
                              <th className="py-2.5 px-4 text-center">Enabled</th>
                              <th className="py-2.5 px-4 text-right">Actions</th>
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-slate-800/40 text-xs">
                            {groupRules.map((rule) => {
                              const runningCount = runningCounts[rule.id] || 0;
                              const isRunning = group.enabled && rule.enabled && runningCount > 0;

                              return (
                                <tr
                                  key={rule.id}
                                  className="hover:bg-slate-800/30 transition-colors group"
                                >
                                  {/* Pattern */}
                                  <td className="py-3 px-4">
                                    <div className="flex flex-col">
                                      <span className="font-mono text-slate-200 font-medium">
                                        {rule.pattern}
                                      </span>
                                      {rule.description && (
                                        <span
                                          className="text-[10px] text-slate-500 truncate max-w-xs"
                                          title={rule.description}
                                        >
                                          {rule.description}
                                        </span>
                                      )}
                                    </div>
                                  </td>

                                  {/* Proxy Target */}
                                  <td className="py-3 px-4">
                                    <div className="flex items-center gap-1.5">
                                      <span
                                        className={`px-1.5 py-0.5 rounded-sm text-[10px] font-mono font-semibold uppercase ${
                                          rule.proxyType === 'http'
                                            ? 'bg-amber-500/10 text-amber-400 border border-amber-500/20'
                                            : 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                                        }`}
                                      >
                                        {rule.proxyType}
                                      </span>
                                      <span className="font-mono text-[11px] text-slate-400">
                                        {rule.proxyTarget}
                                      </span>
                                    </div>
                                  </td>

                                  {/* Live Status */}
                                  <td className="py-3 px-4">
                                    {isRunning ? (
                                      <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                                        <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
                                        Running ({runningCount})
                                      </span>
                                    ) : (
                                      <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-medium bg-slate-800 text-slate-400 border border-slate-700/50">
                                        <span className="w-1.5 h-1.5 rounded-full bg-slate-500" />
                                        {!group.enabled ? 'Group Disabled' : 'Inactive'}
                                      </span>
                                    )}
                                  </td>

                                  {/* Enabled Toggle */}
                                  <td className="py-3 px-4 text-center">
                                    <button
                                      type="button"
                                      onClick={() => handleToggleRule(rule.id)}
                                      className="text-slate-400 hover:text-slate-200 transition-colors cursor-pointer"
                                      title={rule.enabled ? 'Disable Rule' : 'Enable Rule'}
                                    >
                                      {rule.enabled ? (
                                        <ToggleRight className="w-6 h-6 text-indigo-500" />
                                      ) : (
                                        <ToggleLeft className="w-6 h-6 text-slate-600" />
                                      )}
                                    </button>
                                  </td>

                                  {/* Actions */}
                                  <td className="py-3 px-4 text-right">
                                    <div className="flex items-center justify-end gap-1.5">
                                      <button
                                        type="button"
                                        onClick={() => handleOpenEditModal(rule)}
                                        title="Edit Rule"
                                        className="p-1.5 rounded-lg text-slate-400 hover:text-indigo-400 hover:bg-indigo-500/10 transition-colors cursor-pointer"
                                      >
                                        <Pencil className="w-3.5 h-3.5" />
                                      </button>
                                      <button
                                        type="button"
                                        onClick={() => handleDeleteRule(rule.id)}
                                        title="Delete Rule"
                                        className="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-rose-500/10 transition-colors cursor-pointer"
                                      >
                                        <Trash2 className="w-3.5 h-3.5" />
                                      </button>
                                    </div>
                                  </td>
                                </tr>
                              );
                            })}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })
        )}
      </div>

      {/* Active Endpoints Card */}
      <div className="p-5 rounded-2xl bg-slate-900/40 border border-slate-800">
        <h3 className="text-xs font-semibold text-slate-300 flex items-center gap-2 mb-3">
          <Terminal className="w-4 h-4 text-indigo-400" />
          Active Proxy Endpoints
        </h3>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
          <div className="p-3 rounded-xl bg-slate-950 border border-slate-800/80 flex items-center justify-between">
            <div>
              <div className="text-[10px] text-slate-500 font-semibold uppercase">SOCKS5 Endpoint</div>
              <div className="font-mono text-slate-200 mt-0.5">{endpoints.socks5Url}</div>
            </div>
            <button
              type="button"
              onClick={() => copyToClipboard(endpoints.socks5Url, 'socks')}
              className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors cursor-pointer"
            >
              {copiedKey === 'socks' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            </button>
          </div>

          <div className="p-3 rounded-xl bg-slate-950 border border-slate-800/80 flex items-center justify-between">
            <div>
              <div className="text-[10px] text-slate-500 font-semibold uppercase">HTTP Endpoint</div>
              <div className="font-mono text-slate-200 mt-0.5">{endpoints.httpUrl}</div>
            </div>
            <button
              type="button"
              onClick={() => copyToClipboard(endpoints.httpUrl, 'http')}
              className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors cursor-pointer"
            >
              {copiedKey === 'http' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            </button>
          </div>
        </div>
      </div>

      {/* Add / Edit Rule Modal */}
      {isRuleModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-150">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md shadow-2xl overflow-hidden flex flex-col">
            <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-900/50">
              <div>
                <h3 className="text-base font-semibold text-slate-100">
                  {editingRule ? 'Edit Bridge Rule' : 'Add Bridge Rule'}
                </h3>
                {groups.find((g) => g.id === ruleGroupId) && (
                  <p className="text-xs text-slate-400 mt-0.5 flex items-center gap-1.5">
                    <span>Group:</span>
                    <span className="text-indigo-400 font-medium">
                      {groups.find((g) => g.id === ruleGroupId)?.name}
                    </span>
                  </p>
                )}
              </div>
              <button
                type="button"
                onClick={() => setIsRuleModalOpen(false)}
                className="p-1.5 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleSubmitRule} className="p-6 space-y-4">

              {/* Process Pattern */}
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-slate-300">
                  Process Pattern (Wildcard / Regex)
                </label>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={pattern}
                    onChange={(e) => setPattern(e.target.value)}
                    placeholder="e.g. Discovery*.exe, curl, or ^Discord.*"
                    className="flex-1 px-3.5 py-2 rounded-xl bg-slate-950 border border-slate-800 text-slate-100 text-xs font-mono placeholder:text-slate-600 focus:outline-hidden focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-all"
                    autoFocus
                  />
                  <button
                    type="button"
                    onClick={() => setIsSelectAppOpen(true)}
                    className="px-3 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium flex items-center gap-1.5 transition-colors border border-slate-700/50 shrink-0 cursor-pointer"
                    title="Browse installed apps or files"
                  >
                    <AppWindow className="w-3.5 h-3.5 text-indigo-400" />
                    Browse
                  </button>
                </div>
                <p className="text-[11px] text-slate-500">
                  Executable name, supports wildcards (*, ?) or regex.
                </p>
              </div>

              {/* Protocol */}
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-slate-300">Proxy Protocol</label>
                <div className="grid grid-cols-2 gap-2">
                  <button
                    type="button"
                    onClick={() => setProxyType('socks5')}
                    className={`px-3 py-2 rounded-xl text-xs font-medium border transition-colors cursor-pointer ${
                      proxyType === 'socks5'
                        ? 'bg-indigo-600 text-white border-indigo-500'
                        : 'bg-slate-950 text-slate-400 border-slate-800 hover:bg-slate-800 hover:text-slate-200'
                    }`}
                  >
                    SOCKS5 (127.0.0.1:{endpoints.socks5Port})
                  </button>
                  <button
                    type="button"
                    onClick={() => setProxyType('http')}
                    className={`px-3 py-2 rounded-xl text-xs font-medium border transition-colors cursor-pointer ${
                      proxyType === 'http'
                        ? 'bg-indigo-600 text-white border-indigo-500'
                        : 'bg-slate-950 text-slate-400 border-slate-800 hover:bg-slate-800 hover:text-slate-200'
                    }`}
                  >
                    HTTP (127.0.0.1:{endpoints.httpPort})
                  </button>
                </div>
              </div>

              {/* Custom Target */}
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-slate-300 flex items-center justify-between">
                  <span>Custom Proxy Target</span>
                  <span className="text-[10px] text-slate-500">Optional</span>
                </label>
                <input
                  type="text"
                  value={customTarget}
                  onChange={(e) => setCustomTarget(e.target.value)}
                  placeholder={
                    proxyType === 'http'
                      ? `http://127.0.0.1:${endpoints.httpPort}`
                      : `socks5://127.0.0.1:${endpoints.socks5Port}`
                  }
                  className="w-full px-3.5 py-2 rounded-xl bg-slate-950 border border-slate-800 text-slate-100 text-xs font-mono placeholder:text-slate-600 focus:outline-hidden focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-all"
                />
              </div>

              {/* Description */}
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-slate-300 flex items-center justify-between">
                  <span>Description / Executable Path</span>
                  <span className="text-[10px] text-slate-500">Optional</span>
                </label>
                <input
                  type="text"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="e.g. C:\Games\Discovery\Discovery.exe"
                  className="w-full px-3.5 py-2 rounded-xl bg-slate-950 border border-slate-800 text-slate-100 text-xs placeholder:text-slate-600 focus:outline-hidden focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-all"
                />
              </div>

              <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-800/80">
                <button
                  type="button"
                  onClick={() => setIsRuleModalOpen(false)}
                  className="px-4 py-2 rounded-xl text-xs font-medium text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={!pattern.trim()}
                  className="px-5 py-2 rounded-xl text-xs font-medium text-white bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 disabled:pointer-events-none transition-colors shadow-sm cursor-pointer"
                >
                  {editingRule ? 'Save Changes' : 'Add Rule'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Add / Edit Group Modal */}
      {isGroupModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-150">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-sm shadow-2xl overflow-hidden flex flex-col">
            <div className="flex items-center justify-between px-6 py-4 border-b border-slate-800 bg-slate-900/50">
              <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
                <Folder className="w-4 h-4 text-indigo-400" />
                {editingGroup ? 'Rename Group' : 'Create Rule Group'}
              </h3>
              <button
                type="button"
                onClick={() => setIsGroupModalOpen(false)}
                className="p-1.5 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleSubmitGroup} className="p-6 space-y-4">
              <div className="space-y-1.5">
                <label className="text-xs font-medium text-slate-300">Group Name</label>
                <input
                  type="text"
                  value={groupName}
                  onChange={(e) => setGroupName(e.target.value)}
                  placeholder="e.g. Browsers, Work Apps, Games"
                  className="w-full px-3.5 py-2 rounded-xl bg-slate-950 border border-slate-800 text-slate-100 text-xs placeholder:text-slate-600 focus:outline-hidden focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-all"
                  autoFocus
                />
              </div>

              <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-800/80">
                <button
                  type="button"
                  onClick={() => setIsGroupModalOpen(false)}
                  className="px-4 py-2 rounded-xl text-xs font-medium text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={!groupName.trim()}
                  className="px-5 py-2 rounded-xl text-xs font-medium text-white bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 disabled:pointer-events-none transition-colors shadow-sm cursor-pointer"
                >
                  {editingGroup ? 'Save Changes' : 'Create Group'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Delete Group Confirmation Modal */}
      {groupToDelete && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-in fade-in duration-150">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-sm shadow-2xl overflow-hidden flex flex-col p-6 space-y-4">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-xl bg-rose-500/10 border border-rose-500/20 flex items-center justify-center text-rose-400 shrink-0">
                <AlertTriangle className="w-5 h-5" />
              </div>
              <div>
                <h3 className="text-sm font-bold text-slate-100">Delete Rule Group?</h3>
                <p className="text-xs text-slate-400 mt-0.5">
                  Are you sure you want to remove <span className="text-slate-200 font-semibold">"{groupToDelete.name}"</span>?
                </p>
              </div>
            </div>

            <p className="text-xs text-rose-400/90 bg-rose-500/10 border border-rose-500/20 p-2.5 rounded-xl">
              All rules inside this group ({rules.filter((r) => r.groupId === groupToDelete.id).length} rule(s)) will also be permanently deleted.
            </p>

            <div className="flex items-center justify-end gap-2.5 pt-2">
              <button
                type="button"
                onClick={() => setGroupToDelete(null)}
                className="px-4 py-2 rounded-xl text-xs font-medium text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleConfirmDeleteGroup}
                className="px-4 py-2 rounded-xl text-xs font-medium text-white bg-rose-600 hover:bg-rose-500 transition-colors shadow-sm cursor-pointer"
              >
                Delete Group
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Select Installed App Modal */}
      <SelectAppModal
        isOpen={isSelectAppOpen}
        onClose={() => setIsSelectAppOpen(false)}
        onSelectApp={handleSelectAppForForm}
        onBrowseManual={handleBrowseManualForForm}
      />
    </div>
  );
};
