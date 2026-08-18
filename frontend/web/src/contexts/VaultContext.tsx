import React, { createContext, useContext, useState, useEffect, useCallback, useRef, ReactNode } from 'react';
import { useAuth } from './AuthContext';
import * as vaultCrypto from '../crypto/vault';
import type { UnlockedVault } from '../crypto/vault';
import { PrfCancelledError, PrfUnsupportedError, evaluatePrf, evaluatePrfForCredential } from '../crypto/prf';
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
  /** Unlocks using an enrolled passkey via the WebAuthn PRF extension. */
  unlockWithPasskey: () => Promise<boolean>;
  /** True when this vault has at least one passkey enrolled for unlock. */
  passkeyUnlockAvailable: boolean;
  /** Enrols a passkey as an additional unlock factor. Requires an open vault. */
  enrollPasskeyUnlock: (credentialId: string, label?: string) => Promise<void>;
  /** Removes a passkey unlock factor. The master password always remains. */
  revokePasskeyUnlock: (credentialId: string) => Promise<void>;
  /** Credential IDs currently enrolled for unlock. */
  enrolledPasskeys: string[];
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

const EMPTY_VAULT: VaultData = { entries: [], folders: [] };

export function VaultProvider({ children }: { children: ReactNode }) {
  const { token, sessionMode } = useAuth();

  const [isLocked, setIsLocked] = useState(true);
  // The unlocked session: the vault's data encryption key plus its envelope.
  // A ref, not state, for the same reason as the version below — a save must
  // read it synchronously. Holding the DEK here means the master password is
  // needed exactly once, at unlock, and is never retained afterwards.
  const sessionRef = useRef<UnlockedVault<VaultData> | null>(null);
  const [vaultData, setVaultData] = useState<VaultData | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  // Optimistic-concurrency version from the server. A ref, not state: saves
  // must read the current value synchronously, and a re-render must not be
  // required for the next save to send the right one.
  const vaultVersionRef = useRef<number>(0);
  const [lastActivityTime, setLastActivityTime] = useState(Date.now());
  // Passkey enrolments, readable from the stored blob without unlocking it, so
  // the lock screen can offer passkey unlock before any key material exists.
  const [enrolledPasskeys, setEnrolledPasskeys] = useState<string[]>([]);
  const lockedBlobRef = useRef<string | null>(null);

  // Defined before the effects below because the auto-lock effect lists it as a
  // dependency, and a dependency array is evaluated during render — a `const`
  // declared further down would still be in its temporal dead zone.
  const openSession = (session: UnlockedVault<VaultData>) => {
    sessionRef.current = session;
    setVaultData(session.data);
    setIsLocked(false);
    setLastActivityTime(Date.now());
    setEnrolledPasskeys(Object.keys(vaultCrypto.passkeyUnlockSalts(vaultCrypto.serialize(session))));
  };

  const lock = useCallback(() => {
    if (sessionRef.current) {
      // Zero the DEK rather than just dropping the reference.
      vaultCrypto.wipe(sessionRef.current);
      sessionRef.current = null;
    }
    setVaultData(null);
    setIsLocked(true);
  }, []);

  // Auto-lock after inactivity
  useEffect(() => {
    if (isLocked) return;

    const checkInactivity = () => {
      const now = Date.now();
      if (now - lastActivityTime > AUTO_LOCK_TIMEOUT_MS) {
        lock();
        toast('Vault locked due to inactivity', { icon: '🔒' });
      }
    };

    const interval = setInterval(checkInactivity, 60000); // Check every minute
    return () => clearInterval(interval);
  }, [isLocked, lastActivityTime, lock]);

  // While locked, peek at the stored blob to see whether any passkey is
  // enrolled. This reads only the envelope's wrap metadata — no key material is
  // involved and nothing can be decrypted from it.
  useEffect(() => {
    if (!isLocked || (!token && sessionMode !== 'signer')) return;

    let cancelled = false;
    (async () => {
      try {
        const response = await axios.get('/vault');
        if (cancelled) return;
        const blob: string | null = response.data?.encrypted_data ?? null;
        lockedBlobRef.current = blob;
        setEnrolledPasskeys(Object.keys(vaultCrypto.passkeyUnlockSalts(blob)));
      } catch {
        // No vault yet, or the fetch failed. Passkey unlock stays hidden and
        // the master password path is unaffected.
        if (!cancelled) {
          lockedBlobRef.current = null;
          setEnrolledPasskeys([]);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [isLocked, token, sessionMode]);

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

  /**
   * Fetches the stored blob and opens it with the master password.
   *
   * Returns null only when the password is wrong; a genuinely absent vault
   * yields a fresh empty one.
   */
  const loadVault = async (password: string): Promise<UnlockedVault<VaultData> | null> => {
    let encryptedData: string | null = null;

    try {
      const response = await axios.get('/vault');
      const { encrypted_data, version } = response.data;

      // The server uses `version` for optimistic concurrency and REQUIRES it
      // back on PUT. It was never read here, so every save omitted it and the
      // API rejected the whole request with 400 — no item could ever be added.
      vaultVersionRef.current = typeof version === 'number' ? version : 0;
      encryptedData = encrypted_data ?? null;
    } catch (error: any) {
      if (error.response?.status !== 404) {
        throw error;
      }
      // No vault exists yet — fall through and create one.
      vaultVersionRef.current = 0;
    }

    try {
      return await vaultCrypto.unlockWithPassword<VaultData>(encryptedData, password, EMPTY_VAULT);
    } catch (error) {
      if (error instanceof vaultCrypto.VaultFormatError) {
        throw error;
      }
      // Any other failure here is a failed unwrap: wrong password, or a blob
      // that no longer authenticates.
      return null;
    }
  };

  /** Persists the current envelope. Assumes `session` is the live session. */
  const persist = async (session: UnlockedVault<VaultData>): Promise<void> => {
    const response = await axios.put('/vault', {
      encrypted_data: vaultCrypto.serialize(session),
      version: vaultVersionRef.current,
    });
    // Track the server's new version so consecutive saves in one session do
    // not send a stale one. Without this only the first save would line up.
    if (typeof response.data?.version === 'number') {
      vaultVersionRef.current = response.data.version;
    }
  };

  const saveVault = async (data: VaultData): Promise<void> => {
    const session = sessionRef.current;
    if (!session) {
      throw new Error('Vault is locked');
    }

    setSaving(true);
    try {
      // Re-encrypts the body under the existing DEK — no scrypt, and every
      // enrolled unlock factor survives untouched.
      const updated = await vaultCrypto.saveVault(session, data);
      await persist(updated);
      sessionRef.current = updated;
      setVaultData(data);
    } finally {
      setSaving(false);
    }
  };

  const unlock = async (password: string): Promise<boolean> => {
    if (!token && sessionMode !== 'signer') {
      toast.error('Please log in first');
      return false;
    }

    setLoading(true);
    try {
      const session = await loadVault(password);

      if (session === null) {
        toast.error('Invalid master password');
        return false;
      }

      openSession(session);

      if (session.migrated) {
        // The vault was still in the v1 format. It has been re-keyed in memory;
        // write it back now so the upgrade is not repeated on every unlock and
        // so a passkey can be enrolled against it. Failure is non-fatal — the
        // v1 blob on the server is untouched and still opens with this
        // password, so the migration simply retries next time.
        try {
          await persist(session);
          toast.success('Vault unlocked and upgraded to authenticated encryption');
        } catch (error) {
          console.error('Vault format upgrade could not be saved:', error);
          toast('Vault unlocked — encryption upgrade will retry next time', { icon: '⚠️' });
        }
      } else {
        toast.success('Vault unlocked');
      }
      return true;
    } catch (error: any) {
      console.error('Unlock error:', error);
      toast.error(
        error instanceof vaultCrypto.VaultFormatError
          ? 'This vault is in an unrecognised format'
          : 'Failed to unlock vault'
      );
      return false;
    } finally {
      setLoading(false);
    }
  };

  const unlockWithPasskey = async (): Promise<boolean> => {
    if (!token && sessionMode !== 'signer') {
      toast.error('Please log in first');
      return false;
    }

    setLoading(true);
    try {
      // Prefer the blob the lock-screen effect already fetched; fall back to a
      // fresh read if it is not there yet.
      let blob = lockedBlobRef.current;
      if (!blob) {
        const response = await axios.get('/vault');
        blob = response.data?.encrypted_data ?? null;
        vaultVersionRef.current =
          typeof response.data?.version === 'number' ? response.data.version : 0;
        lockedBlobRef.current = blob;
      }

      const salts = vaultCrypto.passkeyUnlockSalts(blob);
      if (!blob || Object.keys(salts).length === 0) {
        toast.error('No passkey is enrolled for this vault');
        return false;
      }

      // One assertion: the authenticator evaluates the PRF and we derive the
      // key-encryption key from the result. The signature itself is discarded.
      const { credentialId, prfOutput } = await evaluatePrf(salts);
      const session = await vaultCrypto.unlockWithPrf<VaultData>(blob, credentialId, prfOutput);

      // Re-read the version: the peek effect does not record it.
      const current = await axios.get('/vault');
      vaultVersionRef.current =
        typeof current.data?.version === 'number' ? current.data.version : vaultVersionRef.current;

      openSession(session);
      toast.success('Vault unlocked with passkey');
      return true;
    } catch (error: any) {
      if (error instanceof PrfCancelledError) {
        // User dismissed the prompt — not an error worth shouting about.
        return false;
      }
      console.error('Passkey unlock error:', error);
      toast.error(
        error instanceof PrfUnsupportedError || error instanceof vaultCrypto.VaultFormatError
          ? error.message
          : 'Passkey unlock failed — use your master password'
      );
      return false;
    } finally {
      setLoading(false);
    }
  };

  const enrollPasskeyUnlock = async (credentialId: string, label?: string): Promise<void> => {
    const session = sessionRef.current;
    if (!session) {
      throw new Error('Unlock the vault before enrolling a passkey');
    }

    const salt = vaultCrypto.newPrfSalt();
    const prfOutput = await evaluatePrfForCredential(credentialId, salt);

    // Wraps the existing DEK for this passkey. The vault body is not touched,
    // and the master password wrap stays exactly as it was.
    const updated = await vaultCrypto.enrollPasskey(session, credentialId, salt, prfOutput, label);
    await persist(updated);
    sessionRef.current = updated;
    setEnrolledPasskeys(Object.keys(vaultCrypto.passkeyUnlockSalts(vaultCrypto.serialize(updated))));
  };

  const revokePasskeyUnlock = async (credentialId: string): Promise<void> => {
    const session = sessionRef.current;
    if (!session) {
      throw new Error('Unlock the vault before changing passkey unlock');
    }

    const updated = vaultCrypto.revokePasskey(session, credentialId);
    await persist(updated);
    sessionRef.current = updated;
    setEnrolledPasskeys(Object.keys(vaultCrypto.passkeyUnlockSalts(vaultCrypto.serialize(updated))));
  };

  const addEntry = async (entryData: Omit<VaultEntry, 'id' | 'created_at' | 'updated_at'>): Promise<void> => {
    if (!vaultData || !sessionRef.current) {
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
    if (!vaultData || !sessionRef.current) {
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
    if (!vaultData || !sessionRef.current) {
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
    if (!vaultData || !sessionRef.current) {
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
    if (!vaultData || !sessionRef.current) {
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
    if (!vaultData || !sessionRef.current) {
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
    unlockWithPasskey,
    passkeyUnlockAvailable: enrolledPasskeys.length > 0,
    enrollPasskeyUnlock,
    revokePasskeyUnlock,
    enrolledPasskeys,
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
