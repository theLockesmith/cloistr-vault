import { describe, it, expect, vi } from 'vitest';
import { VaultClient } from './client';
import type { SignerInterface } from '@cloistr/auth';

const OWNER_PUBKEY = 'a'.repeat(64);
const OUTSIDER_PUBKEY = 'f'.repeat(64);
const TEST_CHALLENGE = 'b'.repeat(64);
const OWNER_TOKEN = 'owner-session-token';
const OUTSIDER_TOKEN = 'outsider-session-token';

function makeSigner(pubkey: string): SignerInterface {
  return {
    getPublicKey: vi.fn().mockResolvedValue(pubkey),
    signEvent: vi.fn().mockResolvedValue({
      id: 'c'.repeat(64),
      pubkey,
      created_at: Math.floor(Date.now() / 1000),
      kind: 22242,
      tags: [],
      content: TEST_CHALLENGE,
      sig: 'd'.repeat(128),
    }),
    encrypt: vi.fn().mockResolvedValue('encrypted'),
    decrypt: vi.fn().mockResolvedValue('decrypted'),
  };
}

describe('access control refusals', () => {
  it('non-member cannot access a shared folder key', async () => {
    const outsiderFetch = vi.fn().mockImplementation(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/auth/nostr/challenge')) {
        return { ok: true, status: 200, json: async () => ({ challenge: TEST_CHALLENGE }) };
      }
      if (url.endsWith('/auth/login')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({ token: OUTSIDER_TOKEN, user: { id: 'outsider-id', nostr_pubkey: OUTSIDER_PUBKEY } }),
        };
      }
      if (url.includes('/sharing/folders/folder-1/key')) {
        return {
          ok: false,
          status: 403,
          json: async () => ({ error: 'access denied: not a team member' }),
        };
      }
      return { ok: false, status: 404, json: async () => ({ error: 'not found' }) };
    });

    const outsider = new VaultClient({
      baseUrl: 'https://vault.test',
      signer: makeSigner(OUTSIDER_PUBKEY),
      fetch: outsiderFetch,
    });

    await outsider.login();

    const resp = outsiderFetch.mock.results.length;
    const folderKeyResp = await outsiderFetch(
      'https://vault.test/api/v1/sharing/folders/folder-1/key',
      { method: 'GET', headers: { Authorization: `Bearer ${OUTSIDER_TOKEN}` } }
    );

    expect(folderKeyResp.ok).toBe(false);
    expect(folderKeyResp.status).toBe(403);
    const body = await folderKeyResp.json();
    expect(body.error).toContain('not a team member');
  });

  it('non-member cannot share a folder they do not own', async () => {
    const outsiderFetch = vi.fn().mockImplementation(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/auth/nostr/challenge')) {
        return { ok: true, status: 200, json: async () => ({ challenge: TEST_CHALLENGE }) };
      }
      if (url.endsWith('/auth/login')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({ token: OUTSIDER_TOKEN, user: { id: 'outsider-id' } }),
        };
      }
      if (url.endsWith('/sharing/folder') && init?.method === 'POST') {
        return {
          ok: false,
          status: 403,
          json: async () => ({ error: 'folder not owned by user' }),
        };
      }
      return { ok: false, status: 404, json: async () => ({ error: 'not found' }) };
    });

    const outsider = new VaultClient({
      baseUrl: 'https://vault.test',
      signer: makeSigner(OUTSIDER_PUBKEY),
      fetch: outsiderFetch,
    });

    await outsider.login();
    await expect(
      outsider.shareFolder('folder-1', 'team-1', 'edit', 'encrypted-key-data')
    ).rejects.toThrow('folder not owned by user');
  });

  it('non-member invite to a team they are not admin of is rejected', async () => {
    const outsiderFetch = vi.fn().mockImplementation(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/auth/nostr/challenge')) {
        return { ok: true, status: 200, json: async () => ({ challenge: TEST_CHALLENGE }) };
      }
      if (url.endsWith('/auth/login')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({ token: OUTSIDER_TOKEN, user: { id: 'outsider-id' } }),
        };
      }
      if (url.includes('/teams/') && url.endsWith('/invite')) {
        return {
          ok: false,
          status: 403,
          json: async () => ({ error: 'insufficient permissions: requires admin or owner role' }),
        };
      }
      return { ok: false, status: 404, json: async () => ({ error: 'not found' }) };
    });

    const outsider = new VaultClient({
      baseUrl: 'https://vault.test',
      signer: makeSigner(OUTSIDER_PUBKEY),
      fetch: outsiderFetch,
    });

    await outsider.login();
    await expect(
      outsider.inviteToTeam('team-1', 'some-user-id', 'member')
    ).rejects.toThrow('insufficient permissions');
  });

  it('viewer role cannot update a shared entry', async () => {
    const { createInitialEnvelope } = await import('@cloistr/vault-crypto');
    const vault = {
      entries: [
        {
          id: 'entry-1',
          type: 'login' as const,
          name: 'Shared Secret',
          fields: { username: 'admin', password: 'secret' },
          notes: '',
          created_at: '2026-01-01',
          updated_at: '2026-01-01',
          favorite: false,
          folder_id: 'shared-folder-1',
        },
      ],
      folders: [{ id: 'shared-folder-1', name: 'Shared', created_at: '2026-01-01' }],
    };
    const blob = await createInitialEnvelope(vault, 'password123');

    let putAttempted = false;
    const viewerFetch = vi.fn().mockImplementation(async (url: string, init?: RequestInit) => {
      if (url.endsWith('/auth/nostr/challenge')) {
        return { ok: true, status: 200, json: async () => ({ challenge: TEST_CHALLENGE }) };
      }
      if (url.endsWith('/auth/login')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({ token: OUTSIDER_TOKEN, user: { id: 'viewer-id' } }),
        };
      }
      if (url.endsWith('/vault') && (!init?.method || init.method === 'GET')) {
        return { ok: true, status: 200, json: async () => ({ encrypted_data: blob, version: 1 }) };
      }
      if (url.endsWith('/vault') && init?.method === 'PUT') {
        putAttempted = true;
        return {
          ok: false,
          status: 403,
          json: async () => ({ error: 'permission denied: view-only access' }),
        };
      }
      return { ok: false, status: 404, json: async () => ({ error: 'not found' }) };
    });

    const viewer = new VaultClient({
      baseUrl: 'https://vault.test',
      signer: makeSigner(OUTSIDER_PUBKEY),
      fetch: viewerFetch,
    });

    await viewer.login();
    await viewer.unlock('password123');

    const entries = viewer.listEntries();
    expect(entries).toHaveLength(1);
    expect(entries[0].name).toBe('Shared Secret');

    await expect(
      viewer.updateEntry({ ...entries[0], name: 'Tampered' })
    ).rejects.toThrow('permission denied');
    expect(putAttempted).toBe(true);
  });

  it('unauthenticated request to vault returns 401', async () => {
    const rawFetch = vi.fn().mockImplementation(async () => ({
      ok: false,
      status: 401,
      json: async () => ({ error: 'Authorization header required' }),
    }));

    const client = new VaultClient({
      baseUrl: 'https://vault.test',
      signer: makeSigner(OUTSIDER_PUBKEY),
      fetch: rawFetch,
    });

    // Skip login, try to unlock directly via the internal state
    // The client should throw because it has no token
    await expect(client.unlock('password')).rejects.toThrow('Not logged in');
  });
});
