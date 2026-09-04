import { useState, useEffect, useCallback } from 'react';
import { api } from '../api/wails';
import type { ConnectionDTO, StatsDTO } from '../types';

interface UseConnectionsOptions {
  showToast: (message: string, type?: 'success' | 'error' | 'info') => void;
}

export function useConnections({ showToast }: UseConnectionsOptions) {
  const [connections, setConnections] = useState<ConnectionDTO[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [activeStats, setActiveStats] = useState<StatsDTO | null>(null);
  const [connectingId, setConnectingId] = useState<string | null>(null);
  const [disconnectingId, setDisconnectingId] = useState<string | null>(null);

  const loadConnections = useCallback(async () => {
    try {
      const items = await api.getConnections();
      setConnections(items);
      setSelectedId(prev => {
        if (prev === null && items.length > 0) {
          const activeItem = items.find(i => i.active);
          return activeItem ? activeItem.id : items[0].id;
        }
        return prev;
      });
    } catch (e) {
      console.error('Failed to load connections:', e);
    }
  }, []);

  useEffect(() => {
    loadConnections();

    const unsubChanged = api.onConnectionsChanged(items => {
      setConnections(items);
    });

    const unsubTick = api.onStatsTick(stats => {
      setActiveStats(stats);
    });

    const unsubStatus = api.onConnectionStatus(event => {
      if (event.status === 'reconnecting') {
        setConnectingId(event.id);
        showToast(event.message || 'Waking from sleep: Reconnecting secure VPN...', 'info');
      } else if (event.status === 'connected') {
        setConnectingId(null);
        setDisconnectingId(null);
        setConnections(prev =>
          prev.map(c => ({
            ...c,
            active: c.id === event.id,
          }))
        );
        showToast('Connected to VPN successfully!', 'success');
      } else if (event.status === 'disconnected') {
        setConnectingId(null);
        setDisconnectingId(null);
        setConnections(prev =>
          prev.map(c => ({
            ...c,
            active: false,
          }))
        );
        showToast('VPN disconnected', 'info');
      } else if (event.status === 'error') {
        setConnectingId(null);
        setDisconnectingId(null);
        setConnections(prev =>
          prev.map(c => ({
            ...c,
            active: false,
          }))
        );
        showToast(`Connection error: ${event.error || 'Unknown error'}`, 'error');
      }
      loadConnections();
    });

    return () => {
      unsubChanged();
      unsubTick();
      unsubStatus();
    };
  }, [loadConnections, showToast]);

  const activeConnection = connections.find(c => c.active);
  const selectedConnection = connections.find(c => c.id === selectedId) || connections[0];

  const handleDisconnect = async (id?: string) => {
    const targetId = id ?? activeConnection?.id ?? null;
    setDisconnectingId(targetId);
    try {
      await api.disconnect();
      await loadConnections();
    } catch (err: any) {
      showToast(err?.message || String(err) || 'Failed to disconnect', 'error');
    } finally {
      setDisconnectingId(null);
    }
  };

  const handleConnect = async (id: string) => {
    const item = connections.find(c => c.id === id);
    if (item?.active) {
      await handleDisconnect(id);
      return;
    }

    setConnectingId(id);
    try {
      await api.connect(id);
      await loadConnections();
      setSelectedId(id);
    } catch (err: any) {
      showToast(err?.message || String(err) || 'Failed to connect', 'error');
    } finally {
      setConnectingId(null);
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await api.deleteConnection(id);
      showToast('Profile deleted', 'info');
      await loadConnections();
      if (selectedId === id) {
        setSelectedId(null);
      }
    } catch (err: any) {
      showToast(err?.message || String(err), 'error');
    }
  };

  const handleResetTraffic = async (id: string) => {
    try {
      await api.resetTraffic(id);
      showToast('Profile traffic reset to 0', 'info');
      await loadConnections();
    } catch (err: any) {
      showToast('Failed to reset traffic: ' + (err?.message || String(err)), 'error');
    }
  };

  const handleReorder = async (sourceIndex: number, targetIndex: number) => {
    const updated = [...connections];
    const [movedItem] = updated.splice(sourceIndex, 1);
    updated.splice(targetIndex, 0, movedItem);
    setConnections(updated);

    try {
      await api.reorderConnections(sourceIndex, targetIndex);
      await loadConnections();
    } catch (err: any) {
      showToast(err?.message || 'Failed to reorder profiles', 'error');
      await loadConnections();
    }
  };

  return {
    connections,
    selectedId,
    setSelectedId,
    activeStats,
    setActiveStats,
    connectingId,
    setConnectingId,
    disconnectingId,
    setDisconnectingId,
    activeConnection,
    selectedConnection,
    loadConnections,
    handleConnect,
    handleDisconnect,
    handleDelete,
    handleResetTraffic,
    handleReorder,
  };
}
