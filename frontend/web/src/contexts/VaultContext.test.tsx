import React, { ReactNode } from 'react';
import { renderHook, act, waitFor } from '@testing-library/react';
import CryptoJS from 'crypto-js';
import axios from 'axios';
import { VaultProvider, useVault } from './VaultContext';
import { isEnvelopeV2 } from '../crypto/envelope';
import { isLegacyVault } from '../crypto/legacy';

jest.mock('axios');
jest.mock('react-hot-toast', () => {
  const toast: any = jest.fn();
  toast.success = jest.fn();
  toast.error = jest.fn();
  return { __esModule: true, default: toast };
});
jest.mock('./AuthContext', () => ({
  useAuth: () => ({ token: 'test-token', sessionMode: 'password' }),
}));

const mockedAxios = axios as jest.Mocked<typeof axios>;

const PASSWORD = 'a-master-password';

const SAMPLE = {
  entries: [
    {
      id: '1',
      type: 'login' as const,
      name: 'email',
      fields: { password: 'hunter2' },
      notes: '',
      created_at: '2026-01-01T00:00:00.000Z',
      updated_at: '2026-01-01T00:00:00.000Z',
      favorite: false,
    },
  ],
  folders: [],
};

/** Builds a blob in the exact format the shipped v1 CryptoContext produced. */
function makeLegacyBlob(vault: unknown, password: string): string {
  const salt = CryptoJS.lib.WordArray.random(256 / 8).toString();
  const key = CryptoJS.PBKDF2(password, salt, {
    keySize: 256 / 32,
    iterations: 100000,
    hasher: CryptoJS.algo.SHA256,
  }).toString();
  const data = CryptoJS.AES.encrypt(JSON.stringify(vault), key).toString();
  return btoa(JSON.stringify({ salt, data }));
}

const wrapper = ({ children }: { children: ReactNode }) => <VaultProvider>{children}</VaultProvider>;

jest.setTimeout(120_000);

beforeEach(() => {
  jest.clearAllMocks();
});

describe('VaultContext v1 migration', () => {
  it('rewrites a v1 vault as v2 on the server at unlock', async () => {
    const legacy = makeLegacyBlob(SAMPLE, PASSWORD);
    mockedAxios.get.mockResolvedValue({ data: { encrypted_data: legacy, version: 7 } });
    mockedAxios.put.mockResolvedValue({ data: { version: 8 } });

    expect(isLegacyVault(legacy)).toBe(true);

    const { result } = renderHook(() => useVault(), { wrapper });

    await act(async () => {
      const ok = await result.current.unlock(PASSWORD);
      expect(ok).toBe(true);
    });

    await waitFor(() => expect(mockedAxios.put).toHaveBeenCalledTimes(1));

    const body = mockedAxios.put.mock.calls[0][1] as { encrypted_data: string; version: number };
    expect(isEnvelopeV2(body.encrypted_data)).toBe(true);
    // Must send back the version the GET returned, or the API 400s.
    expect(body.version).toBe(7);

    // Data survived the re-key.
    expect(result.current.vaultData).toEqual(SAMPLE);
    expect(result.current.isLocked).toBe(false);
  });

  it('leaves the vault usable when persisting the upgrade fails', async () => {
    const legacy = makeLegacyBlob(SAMPLE, PASSWORD);
    mockedAxios.get.mockResolvedValue({ data: { encrypted_data: legacy, version: 3 } });
    mockedAxios.put.mockRejectedValue(new Error('network down'));

    const { result } = renderHook(() => useVault(), { wrapper });

    await act(async () => {
      expect(await result.current.unlock(PASSWORD)).toBe(true);
    });

    // Unlocked and readable even though the rewrite could not be saved; the
    // v1 blob on the server is untouched, so the migration retries next time.
    expect(result.current.isLocked).toBe(false);
    expect(result.current.vaultData).toEqual(SAMPLE);
  });

  it('does not rewrite a vault that is already v2', async () => {
    // Seed a v2 vault by creating one through the provider.
    mockedAxios.get.mockResolvedValue({ data: { encrypted_data: null, version: 0 } });
    mockedAxios.put.mockResolvedValue({ data: { version: 1 } });

    const seed = renderHook(() => useVault(), { wrapper });
    await act(async () => {
      await seed.result.current.unlock(PASSWORD);
    });
    await act(async () => {
      await seed.result.current.addEntry(SAMPLE.entries[0]);
    });

    const v2Blob = (mockedAxios.put.mock.calls[0][1] as { encrypted_data: string }).encrypted_data;
    expect(isEnvelopeV2(v2Blob)).toBe(true);

    // Now unlock that blob fresh — no migration write should occur.
    jest.clearAllMocks();
    mockedAxios.get.mockResolvedValue({ data: { encrypted_data: v2Blob, version: 1 } });

    const { result } = renderHook(() => useVault(), { wrapper });
    await act(async () => {
      expect(await result.current.unlock(PASSWORD)).toBe(true);
    });

    expect(mockedAxios.put).not.toHaveBeenCalled();
  });

  it('rejects the wrong master password against a v1 vault', async () => {
    mockedAxios.get.mockResolvedValue({
      data: { encrypted_data: makeLegacyBlob(SAMPLE, PASSWORD), version: 1 },
    });

    const { result } = renderHook(() => useVault(), { wrapper });

    await act(async () => {
      expect(await result.current.unlock('wrong-password')).toBe(false);
    });

    expect(result.current.isLocked).toBe(true);
    expect(mockedAxios.put).not.toHaveBeenCalled();
  });
});

describe('VaultContext saving', () => {
  it('creates a vault when the server has none', async () => {
    mockedAxios.get.mockResolvedValue({ data: { encrypted_data: null, version: 0 } });
    mockedAxios.put.mockResolvedValue({ data: { version: 1 } });

    const { result } = renderHook(() => useVault(), { wrapper });
    await act(async () => {
      expect(await result.current.unlock(PASSWORD)).toBe(true);
    });

    // Nothing written until there is something to write.
    expect(mockedAxios.put).not.toHaveBeenCalled();

    await act(async () => {
      await result.current.addEntry(SAMPLE.entries[0]);
    });

    expect(mockedAxios.put).toHaveBeenCalledTimes(1);
    expect(result.current.vaultData?.entries).toHaveLength(1);
  });

  it('tracks the server version across consecutive saves', async () => {
    mockedAxios.get.mockResolvedValue({ data: { encrypted_data: null, version: 4 } });
    mockedAxios.put
      .mockResolvedValueOnce({ data: { version: 5 } })
      .mockResolvedValueOnce({ data: { version: 6 } });

    const { result } = renderHook(() => useVault(), { wrapper });
    await act(async () => {
      await result.current.unlock(PASSWORD);
    });

    await act(async () => {
      await result.current.addEntry(SAMPLE.entries[0]);
    });
    await act(async () => {
      await result.current.addFolder('personal');
    });

    expect((mockedAxios.put.mock.calls[0][1] as any).version).toBe(4);
    expect((mockedAxios.put.mock.calls[1][1] as any).version).toBe(5);
  });

  it('locking clears the vault and blocks further writes', async () => {
    mockedAxios.get.mockResolvedValue({ data: { encrypted_data: null, version: 0 } });
    mockedAxios.put.mockResolvedValue({ data: { version: 1 } });

    const { result } = renderHook(() => useVault(), { wrapper });
    await act(async () => {
      await result.current.unlock(PASSWORD);
    });

    act(() => {
      result.current.lock();
    });

    expect(result.current.isLocked).toBe(true);
    expect(result.current.vaultData).toBeNull();

    await expect(result.current.addEntry(SAMPLE.entries[0])).rejects.toThrow(/locked/i);
  });
});
