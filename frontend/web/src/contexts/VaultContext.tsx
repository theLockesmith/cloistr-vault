import React, { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react';
import { useCrypto } from './CryptoContext';
import { useAuth } from './AuthContext';
import axios from 'axios';
import toast from 'react-hot-toast';

export interface VaultEntry {
  id: string;
  type: 'login' | 'note' | 'card' | 'identity';
  name: string;
  fields: Record<string, string>;
  notes: string;
  created_at: string;
  updated_at: string;
  favorite: boolean;
  folder_id?: string;
}

export interface VaultFolder {
  id: string;
  name: string;
  created_at: string;
}

export interface VaultData {
  entries: VaultEntry[];
  folders: VaultFolder[];
}

interface VaultContextType {
  isLocked: boolean;
  vaultData: VaultData | null;
  loading: boolean;
  saving: boolean;
  unlock: (masterPassword: string) => Promise<boolean>;
  lock: () => void;
  addEntry: (entry: Omit<VaultEntry, 'id' | 'created_at' | 'updated_at'>) => Promise<void>;
  updateEntry: (entry: VaultEntry) => Promise<void>;
  deleteEntry: (id: string) => Promise<void>;
  toggleFavorite: (id: string) => Promise<void>;
  addFolder: (name: string) => Promise<VaultFolder>;
  deleteFolder: (id: string) => Promise<void>;
  lastActivityTime: number;
  resetActivityTimer: () => void;
}

const VaultContext = createContext<VaultContextType | undefined>(undefined);

const AUTO_LOCK_TIMEOUT_MS = 15 * 60 * 1000; // 15 minutes

export function VaultProvider({ children }: { children: ReactNode }) {
  const { encryptVault, decryptVault } = useCrypto();
  const { token } = useAuth();

  const [isLocked, setIsLocked] = useState(true);
  const [masterPassword, setMasterPassword] = useState<string | null>(null);
  const [vaultData, setVaultData] = useState<VaultData | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [lastActivityTime, setLastActivityTime] = useState(Date.now());

  // Auto-lock after inactivity
  useEffect(() => {
    if (isLocked || !masterPassword) return;

    const checkInactivity = () => {
      const now = Date.now();
      if (now - lastActivityTime > AUTO_LOCK_TIMEOUT_MS) {
        lock();
        toast('Vault locked due to inactivity', { icon: '🔒' });
      }
    };

    const interval = setInterval(checkInactivity, 60000); // Check every minute
    return () => clearInterval(interval);
  }, [isLocked, masterPassword, lastActivityTime]);

  // Track user activity
  useEffect(() => {
    if (isLocked) return;

    const handleActivity = () => {
      setLastActivityTime(Date.now());
    };

    window.addEventListener('mousemove', handleActivity, { passive: true });
    window.addEventListener('keydown', handleActivity, { passive: true });
    window.addEventListener('click', handleActivity, { passive: true });
    window.addEventListener('scroll', handleActivity, { passive: true });

    return () => {
      window.removeEventListener('mousemove', handleActivity);
      window.removeEventListener('keydown', handleActivity);
      window.removeEventListener('click', handleActivity);
      window.removeEventListener('scroll', handleActivity);
    };
  }, [isLocked]);

  const resetActivityTimer = useCallback(() => {
    setLastActivityTime(Date.now());
  }, []);

  const loadVault = async (password: string): Promise<VaultData | null> => {
    try {
      const response = await axios.get('/vault');
      const { encrypted_data } = response.data;

      if (!encrypted_data) {
        // No vault exists yet, return empty vault
        return { entries: [], folders: [] };
      }

      const decrypted = decryptVault(encrypted_data, password);
      return decrypted;
    } catch (error: any) {
      if (error.response?.status === 404) {
        // No vault exists yet
        return { entries: [], folders: [] };
      }
      throw error;
    }
  };

  const saveVault = async (data: VaultData): Promise<void> => {
    if (!masterPassword) {
      throw new Error('Vault is locked');
    }

    setSaving(true);
    try {
      const encryptedData = encryptVault(data, masterPassword);
      await axios.put('/vault', { encrypted_data: encryptedData });
      setVaultData(data);
    } finally {
      setSaving(false);
    }
  };

  const unlock = async (password: string): Promise<boolean> => {
    if (!token) {
      toast.error('Please log in first');
      return false;
    }

    setLoading(true);
    try {
      const data = await loadVault(password);

      if (data === null) {
        toast.error('Invalid master password');
        return false;
      }

      setMasterPassword(password);
      setVaultData(data);
      setIsLocked(false);
      setLastActivityTime(Date.now());
      toast.success('Vault unlocked');
      return true;
    } catch (error: any) {
      console.error('Unlock error:', error);
      toast.error('Failed to unlock vault');
      return false;
    } finally {
      setLoading(false);
    }
  };

  const lock = useCallback(() => {
    setMasterPassword(null);
    setVaultData(null);
    setIsLocked(true);
  }, []);

  const addEntry = async (entryData: Omit<VaultEntry, 'id' | 'created_at' | 'updated_at'>): Promise<void> => {
    if (!vaultData || !masterPassword) {
      throw new Error('Vault is locked');
    }

    const now = new Date().toISOString();
    const entry: VaultEntry = {
      ...entryData,
      id: crypto.randomUUID(),
      created_at: now,
      updated_at: now,
    };

    const updatedData = {
      ...vaultData,
      entries: [...vaultData.entries, entry],
    };

    await saveVault(updatedData);
    toast.success('Item added');
    resetActivityTimer();
  };

  const updateEntry = async (entry: VaultEntry): Promise<void> => {
    if (!vaultData || !masterPassword) {
      throw new Error('Vault is locked');
    }

    const updatedEntry = {
      ...entry,
      updated_at: new Date().toISOString(),
    };

    const updatedData = {
      ...vaultData,
      entries: vaultData.entries.map((e) => (e.id === entry.id ? updatedEntry : e)),
    };

    await saveVault(updatedData);
    toast.success('Item updated');
    resetActivityTimer();
  };

  const deleteEntry = async (id: string): Promise<void> => {
    if (!vaultData || !masterPassword) {
      throw new Error('Vault is locked');
    }

    const updatedData = {
      ...vaultData,
      entries: vaultData.entries.filter((e) => e.id !== id),
    };

    await saveVault(updatedData);
    toast.success('Item deleted');
    resetActivityTimer();
  };

  const toggleFavorite = async (id: string): Promise<void> => {
    if (!vaultData || !masterPassword) {
      throw new Error('Vault is locked');
    }

    const entry = vaultData.entries.find((e) => e.id === id);
    if (!entry) return;

    const updatedEntry = {
      ...entry,
      favorite: !entry.favorite,
      updated_at: new Date().toISOString(),
    };

    const updatedData = {
      ...vaultData,
      entries: vaultData.entries.map((e) => (e.id === id ? updatedEntry : e)),
    };

    await saveVault(updatedData);
    resetActivityTimer();
  };

  const addFolder = async (name: string): Promise<VaultFolder> => {
    if (!vaultData || !masterPassword) {
      throw new Error('Vault is locked');
    }

    const folder: VaultFolder = {
      id: crypto.randomUUID(),
      name,
      created_at: new Date().toISOString(),
    };

    const updatedData = {
      ...vaultData,
      folders: [...vaultData.folders, folder],
    };

    await saveVault(updatedData);
    toast.success('Folder created');
    resetActivityTimer();
    return folder;
  };

  const deleteFolder = async (id: string): Promise<void> => {
    if (!vaultData || !masterPassword) {
      throw new Error('Vault is locked');
    }

    // Move all entries from this folder to no folder
    const updatedEntries = vaultData.entries.map((e) =>
      e.folder_id === id ? { ...e, folder_id: undefined } : e
    );

    const updatedData = {
      ...vaultData,
      entries: updatedEntries,
      folders: vaultData.folders.filter((f) => f.id !== id),
    };

    await saveVault(updatedData);
    toast.success('Folder deleted');
    resetActivityTimer();
  };

  const value: VaultContextType = {
    isLocked,
    vaultData,
    loading,
    saving,
    unlock,
    lock,
    addEntry,
    updateEntry,
    deleteEntry,
    toggleFavorite,
    addFolder,
    deleteFolder,
    lastActivityTime,
    resetActivityTimer,
  };

  return <VaultContext.Provider value={value}>{children}</VaultContext.Provider>;
}

export function useVault() {
  const context = useContext(VaultContext);
  if (context === undefined) {
    throw new Error('useVault must be used within a VaultProvider');
  }
  return context;
}
