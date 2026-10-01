import { describe, it, expect, vi, beforeEach } from 'vitest';
import { VaultClient } from './client';
import type { SignerInterface } from '@cloistr/auth';

const TEST_PUBKEY = 'a'.repeat(64);
const TEST_CHALLENGE = 'b'.repeat(64);
const TEST_TOKEN = 'test-session-token-uuid';
const TEST_PASSWORD = 'strong-master-password-123!';

function mockSigner(): SignerInterface {
  return {
    getPublicKey: vi.fn().mockResolvedValue(TEST_PUBKEY),
    signEvent: vi.fn().mockResolvedValue({
      id: 'c'.repeat(64),
      pubkey: TEST_PUBKEY,
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

function mockFetch(responses: Array<{ status: number; body: unknown }>) {
  let callIndex = 0;
  return vi.fn().mockImplementation(async () => {
    const resp = responses[callIndex++] ?? { status: 500, body: { error: 'no mock' } };
    return {
      ok: resp.status >= 200 && resp.status < 300,
      status: resp.status,
      json: async () => resp.body,
    };
  });
}

describe('VaultClient', () => {
  let signer: SignerInterface;

  beforeEach(() => {
    signer = mockSigner();
  });

  describe('login', () => {
    it('gets a challenge, signs it, and logs in', async () => {
      const fetchMock = mockFetch([
        { status: 200, body: { challenge: TEST_CHALLENGE, expires_at: '2099-01-01' } },
        { status: 200, body: { token: TEST_TOKEN, user: { id: 'u1', nostr_pubkey: TEST_PUBKEY } } },
      ]);

      const client = new VaultClient({
        baseUrl: 'https://vault.test',
        signer,
        fetch: fetchMock,
      });

      await client.login();

      expect(fetchMock).toHaveBeenCalledTimes(2);

      const challengeCall = fetchMock.mock.calls[0];
      expect(challengeCall[0]).toBe('https://vault.test/api/v1/auth/nostr/challenge');

      const loginCall = fetchMock.mock.calls[1];
      expect(loginCall[0]).toBe('https://vault.test/api/v1/auth/login');
      const loginBody = JSON.parse(loginCall[1].body);
      expect(loginBody.method).toBe('nostr');
      expect(loginBody.nostr_pubkey).toBe(TEST_PUBKEY);
      expect(loginBody.signed_event).toBeTruthy();
      const parsedEvent = JSON.parse(loginBody.signed_event); expect(parsedEvent.kind).toBe(22242);
    });

    it('throws on challenge failure', async () => {
      const fetchMock = mockFetch([
        { status: 400, body: { error: 'invalid pubkey' } },
      ]);

      const client = new VaultClient({
        baseUrl: 'https://vault.test',
        signer,
        fetch: fetchMock,
      });

      await expect(client.login()).rejects.toThrow('Challenge request failed');
    });
  });

  describe('unlock and entry CRUD', () => {
    it('unlocks a vault and reads entries', async () => {
      const { createInitialEnvelope } = await import('@cloistr/vault-crypto');
      const emptyVault = { entries: [], folders: [] };
      const blob = await createInitialEnvelope(emptyVault, TEST_PASSWORD);

      const fetchMock = mockFetch([
        // login: challenge
        { status: 200, body: { challenge: TEST_CHALLENGE } },
        // login: auth
        { status: 200, body: { token: TEST_TOKEN, user: { id: 'u1' } } },
        // unlock: GET /vault
        { status: 200, body: { encrypted_data: blob, version: 1 } },
      ]);

      const client = new VaultClient({
        baseUrl: 'https://vault.test',
        signer,
        fetch: fetchMock,
      });

      await client.login();
      const entries = await client.unlock(TEST_PASSWORD);
      expect(entries).toEqual([]);
    });

    it('creates an entry and persists', async () => {
      const { createInitialEnvelope } = await import('@cloistr/vault-crypto');
      const emptyVault = { entries: [], folders: [] };
      const blob = await createInitialEnvelope(emptyVault, TEST_PASSWORD);

      let savedBlob = '';
      const fetchMock = vi.fn().mockImplementation(async (url: string, init?: RequestInit) => {
        if (url.endsWith('/auth/nostr/challenge')) {
          return { ok: true, status: 200, json: async () => ({ challenge: TEST_CHALLENGE }) };
        }
        if (url.endsWith('/auth/login')) {
          return { ok: true, status: 200, json: async () => ({ token: TEST_TOKEN, user: { id: 'u1' } }) };
        }
        if (url.endsWith('/vault') && init?.method === 'GET') {
          return { ok: true, status: 200, json: async () => ({ encrypted_data: savedBlob || blob, version: 1 }) };
        }
        if (url.endsWith('/vault') && init?.method === 'PUT') {
          const body = JSON.parse(init.body as string);
          savedBlob = body.encrypted_data;
          return { ok: true, status: 200, json: async () => ({ version: 2 }) };
        }
        return { ok: false, status: 404, json: async () => ({ error: 'not found' }) };
      });

      const client = new VaultClient({
        baseUrl: 'https://vault.test',
        signer,
        fetch: fetchMock,
      });

      await client.login();
      await client.unlock(TEST_PASSWORD);

      await client.createEntry({
        type: 'login',
        name: 'Test Login',
        fields: { username: 'admin', password: 'secret123' },
        notes: '',
        favorite: false,
      });

      const entries = client.listEntries();
      expect(entries).toHaveLength(1);
      expect(entries[0].name).toBe('Test Login');
      expect(entries[0].fields.username).toBe('admin');
      expect(entries[0].id).toBeTruthy();
    });
  });

  describe('team operations', () => {
    it('creates a team', async () => {
      const fetchMock = mockFetch([
        { status: 200, body: { challenge: TEST_CHALLENGE } },
        { status: 200, body: { token: TEST_TOKEN, user: { id: 'u1' } } },
        { status: 201, body: { id: 'team-1', name: 'Fleet Team', owner_id: 'u1' } },
      ]);

      const client = new VaultClient({
        baseUrl: 'https://vault.test',
        signer,
        fetch: fetchMock,
      });

      await client.login();
      const team = await client.createTeam('Fleet Team');
      expect(team.name).toBe('Fleet Team');
      expect(team.id).toBe('team-1');
    });

    it('lists teams', async () => {
      const fetchMock = mockFetch([
        { status: 200, body: { challenge: TEST_CHALLENGE } },
        { status: 200, body: { token: TEST_TOKEN, user: { id: 'u1' } } },
        { status: 200, body: [{ id: 'team-1', name: 'Fleet' }] },
      ]);

      const client = new VaultClient({
        baseUrl: 'https://vault.test',
        signer,
        fetch: fetchMock,
      });

      await client.login();
      const teams = await client.listTeams();
      expect(teams).toHaveLength(1);
    });
  });

  describe('access control', () => {
    it('rejects operations when not logged in', async () => {
      const client = new VaultClient({
        baseUrl: 'https://vault.test',
        signer,
        fetch: vi.fn(),
      });

      await expect(client.unlock(TEST_PASSWORD)).rejects.toThrow('Not logged in');
    });

    it('rejects entry operations when vault is locked', async () => {
      const fetchMock = mockFetch([
        { status: 200, body: { challenge: TEST_CHALLENGE } },
        { status: 200, body: { token: TEST_TOKEN, user: { id: 'u1' } } },
      ]);

      const client = new VaultClient({
        baseUrl: 'https://vault.test',
        signer,
        fetch: fetchMock,
      });

      await client.login();
      expect(() => client.listEntries()).toThrow('Vault is locked');
    });
  });
});
