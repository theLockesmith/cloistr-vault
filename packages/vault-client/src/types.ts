export interface VaultEntry {
  id: string;
  type: 'login' | 'note' | 'card' | 'identity' | 'totp';
  name: string;
  fields: Record<string, string>;
  notes: string;
  created_at: string;
  updated_at: string;
  favorite: boolean;
  folder_id?: string;
  custom_fields?: Array<{ label: string; value: string }>;
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

export interface Team {
  id: string;
  name: string;
  description?: string;
  owner_id: string;
  created_at: string;
  updated_at: string;
}

export interface TeamMember {
  user_id: string;
  team_id: string;
  role: 'owner' | 'admin' | 'member' | 'viewer';
  joined_at: string;
}

export interface SharedFolder {
  id: string;
  folder_id: string;
  team_id?: string;
  shared_by: string;
  shared_with?: string;
  permission_level: 'view' | 'edit' | 'admin';
  created_at: string;
  expires_at?: string;
}

export interface VaultUser {
  id: string;
  email?: string;
  nostr_pubkey?: string;
  display_name?: string;
  auth_method?: string;
}

export type FetchFn = typeof globalThis.fetch;

export interface VaultClientOptions {
  baseUrl: string;
  signer: import('@cloistr/auth').SignerInterface;
  fetch?: FetchFn;
}
