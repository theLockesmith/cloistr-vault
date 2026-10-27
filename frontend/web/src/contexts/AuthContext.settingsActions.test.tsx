/**
 * Actions the Settings page runs must not touch the app-wide `loading` flag.
 * ProtectedRoute shows a spinner in place of the page while `loading` is
 * true, so flipping it unmounts Settings mid-call and the result lands on a
 * component that no longer exists: the NIP-05 lookup never showed its result
 * (found by the WV4 live sweep, VAULT-8).
 */
import React from 'react';
import { renderHook, waitFor, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import axios from 'axios';
import { AuthProvider, useAuth } from './AuthContext';

vi.mock('react-hot-toast', () => {
  const toast: any = vi.fn();
  toast.success = vi.fn();
  toast.error = vi.fn();
  return { __esModule: true, default: toast };
});

function wrapper({ children }: { children: React.ReactNode }) {
  return (
    <MemoryRouter>
      <AuthProvider>{children}</AuthProvider>
    </MemoryRouter>
  );
}

async function renderAuth() {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(
    new Response('{}', { status: 401, headers: { 'Content-Type': 'application/json' } })
  );
  const { result } = renderHook(() => useAuth(), { wrapper });
  await waitFor(() => expect(result.current.loading).toBe(false));
  return result;
}

/** A request that stays pending until the test releases it. */
function pending<T>() {
  let release!: (v: T) => void;
  const promise = new Promise<T>((r) => (release = r));
  return { promise, release };
}

describe('Settings actions leave the app-wide loading flag alone', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it('NIP-05 lookup', async () => {
    const result = await renderAuth();
    const req = pending<any>();
    vi.spyOn(axios, 'get').mockReturnValue(req.promise as any);

    let call!: Promise<any>;
    act(() => {
      call = result.current.lookupNIP05('a@b.c');
    });
    expect(result.current.loading).toBe(false);

    await act(async () => {
      req.release({ data: { nip05_address: 'a@b.c', pubkey: 'p', relays: [] } });
      expect((await call).pubkey).toBe('p');
    });
  });

  it('NIP-05 verify', async () => {
    const result = await renderAuth();
    const req = pending<any>();
    vi.spyOn(axios, 'post').mockReturnValue(req.promise as any);

    let call!: Promise<void>;
    act(() => {
      call = result.current.verifyNIP05('a@b.c');
    });
    expect(result.current.loading).toBe(false);

    await act(async () => {
      req.release({ data: {} });
      await call;
    });
  });

  it('passkey registration', async () => {
    (window as any).PublicKeyCredential = function PublicKeyCredential() {};
    const result = await renderAuth();
    expect(result.current.isWebAuthnAvailable).toBe(true);
    const req = pending<any>();
    vi.spyOn(axios, 'post').mockReturnValue(req.promise as any);

    let call!: Promise<unknown>;
    act(() => {
      call = result.current.registerWebAuthnCredential('key').catch(() => undefined);
    });
    expect(result.current.loading).toBe(false);

    await act(async () => {
      req.release({ data: {} });
      await call;
    });
    delete (window as any).PublicKeyCredential;
  });
});
