import type { SignerInterface } from '@cloistr/auth';
import {
  unlockWithPassword,
  createInitialEnvelope,
  saveVault,
  serialize,
  wipe,
} from '@cloistr/vault-crypto';
import type { UnlockedVault } from '@cloistr/vault-crypto';
import type {
  VaultData,
  VaultEntry,
  VaultUser,
  Team,
  TeamMember,
  SharedFolder,
  FetchFn,
} from './types';

const EMPTY_VAULT: VaultData = { entries: [], folders: [] };

export class VaultClient {
  private baseUrl: string;
  private signer: SignerInterface;
  private fetch: FetchFn;
  private token: string | null = null;
  private user: VaultUser | null = null;
  private session: UnlockedVault<VaultData> | null = null;
  private vaultVersion = 0;

  constructor(opts: { baseUrl: string; signer: SignerInterface; fetch?: FetchFn }) {
    this.baseUrl = opts.baseUrl.replace(/\/$/, '');
    this.signer = opts.signer;
    this.fetch = opts.fetch ?? globalThis.fetch.bind(globalThis);
  }

  private async api(path: string, init?: RequestInit & { method?: string }): Promise<Response> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(this.token ? { Authorization: `Bearer ${this.token}` } : {}),
    };
    return this.fetch(`${this.baseUrl}${path}`, {
      ...init,
      headers: { ...headers, ...(init?.headers as Record<string, string> ?? {}) },
    });
  }

  private requireAuth(): void {
    if (!this.token) throw new Error('Not logged in');
  }

  private requireUnlocked(): UnlockedVault<VaultData> {
    if (!this.session) throw new Error('Vault is locked');
    return this.session;
  }

  async login(): Promise<VaultUser> {
    const pubkey = await this.signer.getPublicKey();

    const challengeResp = await this.api('/api/v1/auth/nostr/challenge', {
      method: 'POST',
      body: JSON.stringify({ public_key: pubkey }),
    });
    if (!challengeResp.ok) {
      throw new Error(`Challenge request failed: ${challengeResp.status}`);
    }
    const { challenge } = await challengeResp.json() as { challenge: string };

    const signedEvent = await this.signer.signEvent({
      kind: 22242,
      tags: [['challenge', challenge]],
      content: challenge,
      created_at: Math.floor(Date.now() / 1000),
    });

    const loginResp = await this.api('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({
        method: 'nostr',
        nostr_pubkey: pubkey,
        signed_event: JSON.stringify(signedEvent),
      }),
    });
    if (!loginResp.ok) {
      const err = await loginResp.json() as { error?: string };
      throw new Error(`Login failed: ${err.error ?? loginResp.status}`);
    }
    const { token, user } = await loginResp.json() as { token: string; user: VaultUser };
    this.token = token;
    this.user = user;
    return user;
  }

  async register(masterPassword: string): Promise<VaultUser> {
    const pubkey = await this.signer.getPublicKey();
    const blob = await createInitialEnvelope(EMPTY_VAULT, masterPassword);

    const resp = await this.api('/api/v1/auth/register', {
      method: 'POST',
      body: JSON.stringify({
        method: 'nostr',
        nostr_pubkey: pubkey,
        vault_data: blob,
      }),
    });
    if (!resp.ok) {
      const err = await resp.json() as { error?: string };
      throw new Error(`Registration failed: ${err.error ?? resp.status}`);
    }
    return this.login();
  }

  async unlock(masterPassword: string): Promise<VaultEntry[]> {
    this.requireAuth();

    const resp = await this.api('/api/v1/vault', { method: 'GET' });
    if (!resp.ok && (resp as Response).status !== 404) {
      throw new Error(`Vault fetch failed: ${resp.status}`);
    }

    let blob: string | null = null;
    if (resp.ok) {
      const data = await resp.json() as { encrypted_data?: string; version?: number };
      blob = data.encrypted_data ?? null;
      this.vaultVersion = typeof data.version === 'number' ? data.version : 0;
    }

    this.session = await unlockWithPassword<VaultData>(blob, masterPassword, EMPTY_VAULT);

    if (this.session.migrated) {
      await this.persist();
    }

    return this.session.data.entries;
  }

  lock(): void {
    if (this.session) {
      wipe(this.session);
      this.session = null;
    }
  }

  private async persist(): Promise<void> {
    const session = this.requireUnlocked();
    const resp = await this.api('/api/v1/vault', {
      method: 'PUT',
      body: JSON.stringify({
        encrypted_data: serialize(session),
        version: this.vaultVersion,
      }),
    });
    if (!resp.ok) {
      const err = await resp.json() as { error?: string };
      throw new Error(`Vault save failed: ${err.error ?? resp.status}`);
    }
    const result = await resp.json() as { version?: number };
    if (typeof result.version === 'number') {
      this.vaultVersion = result.version;
    }
  }

  listEntries(): VaultEntry[] {
    return this.requireUnlocked().data.entries;
  }

  getEntry(id: string): VaultEntry | undefined {
    return this.requireUnlocked().data.entries.find((e) => e.id === id);
  }

  async createEntry(
    entry: Omit<VaultEntry, 'id' | 'created_at' | 'updated_at'>
  ): Promise<VaultEntry> {
    const session = this.requireUnlocked();
    const now = new Date().toISOString();
    const newEntry: VaultEntry = {
      ...entry,
      id: crypto.randomUUID(),
      created_at: now,
      updated_at: now,
    };

    const data: VaultData = {
      ...session.data,
      entries: [...session.data.entries, newEntry],
    };
    this.session = await saveVault(session, data);
    await this.persist();
    return newEntry;
  }

  async updateEntry(entry: VaultEntry): Promise<VaultEntry> {
    const session = this.requireUnlocked();
    const idx = session.data.entries.findIndex((e) => e.id === entry.id);
    if (idx === -1) throw new Error(`Entry not found: ${entry.id}`);

    const updated = { ...entry, updated_at: new Date().toISOString() };
    const entries = [...session.data.entries];
    entries[idx] = updated;

    const data: VaultData = { ...session.data, entries };
    this.session = await saveVault(session, data);
    await this.persist();
    return updated;
  }

  async deleteEntry(id: string): Promise<void> {
    const session = this.requireUnlocked();
    const entries = session.data.entries.filter((e) => e.id !== id);
    if (entries.length === session.data.entries.length) {
      throw new Error(`Entry not found: ${id}`);
    }

    const data: VaultData = { ...session.data, entries };
    this.session = await saveVault(session, data);
    await this.persist();
  }

  async createTeam(name: string, description?: string): Promise<Team> {
    this.requireAuth();
    const resp = await this.api('/api/v1/teams', {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    });
    if (!resp.ok) {
      const err = await resp.json() as { error?: string };
      throw new Error(`Create team failed: ${err.error ?? resp.status}`);
    }
    return resp.json() as Promise<Team>;
  }

  async listTeams(): Promise<Team[]> {
    this.requireAuth();
    const resp = await this.api('/api/v1/teams', { method: 'GET' });
    if (!resp.ok) throw new Error(`List teams failed: ${resp.status}`);
    return resp.json() as Promise<Team[]>;
  }

  async getTeamMembers(teamId: string): Promise<TeamMember[]> {
    this.requireAuth();
    const resp = await this.api(`/api/v1/teams/${teamId}/members`, { method: 'GET' });
    if (!resp.ok) throw new Error(`Get members failed: ${resp.status}`);
    return resp.json() as Promise<TeamMember[]>;
  }

  async inviteToTeam(teamId: string, userId: string, role: string): Promise<void> {
    this.requireAuth();
    const resp = await this.api(`/api/v1/teams/${teamId}/invite`, {
      method: 'POST',
      body: JSON.stringify({ user_id: userId, role }),
    });
    if (!resp.ok) {
      const err = await resp.json() as { error?: string };
      throw new Error(`Invite failed: ${err.error ?? resp.status}`);
    }
  }

  async shareFolder(
    folderId: string,
    teamId: string,
    permissionLevel: 'view' | 'edit' | 'admin',
    encryptedFolderKey: string
  ): Promise<SharedFolder> {
    this.requireAuth();
    const resp = await this.api('/api/v1/sharing/folder', {
      method: 'POST',
      body: JSON.stringify({
        folder_id: folderId,
        team_id: teamId,
        permission_level: permissionLevel,
        encrypted_folder_key: encryptedFolderKey,
      }),
    });
    if (!resp.ok) {
      const err = await resp.json() as { error?: string };
      throw new Error(`Share folder failed: ${err.error ?? resp.status}`);
    }
    return resp.json() as Promise<SharedFolder>;
  }

  async getSharedFolders(): Promise<SharedFolder[]> {
    this.requireAuth();
    const resp = await this.api('/api/v1/sharing/folders', { method: 'GET' });
    if (!resp.ok) throw new Error(`List shared folders failed: ${resp.status}`);
    return resp.json() as Promise<SharedFolder[]>;
  }

  get isLoggedIn(): boolean {
    return this.token !== null;
  }

  get isUnlocked(): boolean {
    return this.session !== null;
  }

  get currentUser(): VaultUser | null {
    return this.user;
  }
}
