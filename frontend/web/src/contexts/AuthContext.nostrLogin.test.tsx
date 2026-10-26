/**
 * Nostr sign-in through a browser extension (NIP-07): the client asks the
 * server for a one-time challenge, has the extension sign a kind 22242 event
 * carrying it, and sends that signed event. The web client used to show an
 * error instead (found by the WV4 live sweep, VAULT-3).
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

const PUBKEY = 'a'.repeat(64);

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

describe('Nostr extension sign-in', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });
  afterEach(() => {
    delete (window as any).nostr;
  });

  it('signs the server challenge as a kind 22242 event and sends it', async () => {
    const signEvent = vi.fn(async (e: any) => ({ ...e, pubkey: PUBKEY, id: 'id', sig: 'sig' }));
    (window as any).nostr = { getPublicKey: async () => PUBKEY, signEvent };
    const result = await renderAuth();
    const post = vi.spyOn(axios, 'post').mockImplementation((url: string) =>
      Promise.resolve(
        url === '/auth/nostr/challenge'
          ? { data: { challenge: 'the-challenge' } }
          : { data: { token: 't', user: { id: 'u' } } }
      ) as any
    );

    await act(() => result.current.loginWithNostr());

    expect(post.mock.calls[0]).toEqual(['/auth/nostr/challenge', { public_key: PUBKEY }]);
    const unsigned = signEvent.mock.calls[0][0];
    expect(unsigned.kind).toBe(22242);
    expect(unsigned.tags).toContainEqual(['challenge', 'the-challenge']);
    expect(Math.abs(unsigned.created_at - Date.now() / 1000)).toBeLessThan(60);

    const [url, body] = post.mock.calls[1] as [string, any];
    expect(url).toBe('/auth/login');
    expect(body.method).toBe('nostr');
    expect(body.nostr_pubkey).toBe(PUBKEY);
    expect(JSON.parse(body.signed_event)).toMatchObject({ kind: 22242, sig: 'sig', pubkey: PUBKEY });
    expect(result.current.token).toBe('t');
  });

  it('says so, and calls nothing, when no extension is installed', async () => {
    const result = await renderAuth();
    const post = vi.spyOn(axios, 'post');

    await act(async () => {
      await expect(result.current.loginWithNostr()).rejects.toThrow(/extension/i);
    });
    expect(post).not.toHaveBeenCalled();
  });
});
