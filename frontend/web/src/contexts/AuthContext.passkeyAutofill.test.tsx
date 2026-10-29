/**
 * Passkey autofill (WebAuthn conditional mediation): the login page keeps a
 * conditional request pending so the browser offers saved passkeys in the
 * email field's autofill list. That request can sit for minutes, so it must
 * not hold the app-wide `loading` flag (which disables every sign-in button),
 * and being cancelled is normal, not an error to show the user.
 */
import React from 'react';
import { renderHook, waitFor, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import axios from 'axios';
import toast from 'react-hot-toast';
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

const credentialsGet = vi.fn();

function stubWebAuthn(conditional: boolean) {
  const PKC: any = function PublicKeyCredential() {};
  PKC.isConditionalMediationAvailable = vi.fn().mockResolvedValue(conditional);
  (window as any).PublicKeyCredential = PKC;
  Object.defineProperty(navigator, 'credentials', {
    configurable: true,
    value: { get: credentialsGet },
  });
}

async function renderAuth() {
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(
    new Response('{}', { status: 401, headers: { 'Content-Type': 'application/json' } })
  );
  const { result } = renderHook(() => useAuth(), { wrapper });
  await waitFor(() => expect(result.current.loading).toBe(false));
  await waitFor(() => expect(result.current.isWebAuthnAvailable).toBe(true));
  return result;
}

// A fresh copy per call: the client decodes the challenge in place.
const begin = () => ({
  data: { options: { publicKey: { challenge: 'AAAA' } }, session_id: 'sess-1' },
});

const buf = (n: number) => new Uint8Array([n]).buffer;
const credential = {
  id: 'cred-1',
  rawId: buf(1),
  type: 'public-key',
  response: {
    authenticatorData: buf(2),
    clientDataJSON: buf(3),
    signature: buf(4),
    userHandle: null,
  },
};

function abortError() {
  return new DOMException('The operation was aborted.', 'AbortError');
}

describe('passkey autofill', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
    credentialsGet.mockReset();
    (toast.error as any).mockClear();
    (toast.success as any).mockClear();
  });

  it('asks for a conditional request tied to the caller\'s abort signal', async () => {
    stubWebAuthn(true);
    const result = await renderAuth();
    vi.spyOn(axios, 'post').mockImplementation(async () => begin() as any);
    credentialsGet.mockRejectedValue(abortError());

    const ac = new AbortController();
    await act(async () => {
      await result.current.startPasskeyAutofill(ac.signal);
    });

    // The challenge request is cancellable too, not just the browser prompt.
    expect(axios.post).toHaveBeenCalledWith(
      '/auth/webauthn/login/begin/discoverable',
      undefined,
      { signal: ac.signal },
    );
    expect(credentialsGet).toHaveBeenCalledTimes(1);
    const opts = credentialsGet.mock.calls[0][0];
    expect(opts.mediation).toBe('conditional');
    expect(opts.signal).toBe(ac.signal);
  });

  it('does not hold the loading flag while waiting for the user', async () => {
    stubWebAuthn(true);
    const result = await renderAuth();
    vi.spyOn(axios, 'post').mockImplementation(async () => begin() as any);
    let reject!: (e: unknown) => void;
    credentialsGet.mockReturnValue(new Promise((_, r) => (reject = r)));

    let call!: Promise<string>;
    act(() => {
      call = result.current.startPasskeyAutofill(new AbortController().signal);
    });
    await waitFor(() => expect(credentialsGet).toHaveBeenCalled());
    expect(result.current.loading).toBe(false);

    await act(async () => {
      reject(abortError());
      await call;
    });
  });

  it('reports a cancelled request as aborted, with no error toast', async () => {
    stubWebAuthn(true);
    const result = await renderAuth();
    vi.spyOn(axios, 'post').mockImplementation(async () => begin() as any);
    credentialsGet.mockRejectedValue(abortError());

    let outcome!: string;
    await act(async () => {
      outcome = await result.current.startPasskeyAutofill(new AbortController().signal);
    });

    expect(outcome).toBe('aborted');
    expect(toast.error).not.toHaveBeenCalled();
  });

  it('does nothing when the browser has no passkey autofill', async () => {
    stubWebAuthn(false);
    const result = await renderAuth();
    const post = vi.spyOn(axios, 'post');

    let outcome!: string;
    await act(async () => {
      outcome = await result.current.startPasskeyAutofill(new AbortController().signal);
    });

    expect(outcome).toBe('unsupported');
    expect(post).not.toHaveBeenCalled();
    expect(credentialsGet).not.toHaveBeenCalled();
  });

  it('signs in when the user picks a passkey from autofill', async () => {
    stubWebAuthn(true);
    const result = await renderAuth();
    const post = vi.spyOn(axios, 'post').mockImplementation(async (url: string) => {
      if (url === '/auth/webauthn/login/begin/discoverable') return begin() as any;
      if (url === '/auth/webauthn/login/finish') {
        return { data: { token: 'tok', user: { id: 'u1', email: 'a@b.c' } } } as any;
      }
      throw new Error('unexpected ' + url);
    });
    credentialsGet.mockResolvedValue(credential);

    let outcome!: string;
    await act(async () => {
      outcome = await result.current.startPasskeyAutofill(new AbortController().signal);
    });

    expect(outcome).toBe('signed-in');
    expect(post).toHaveBeenCalledWith(
      '/auth/webauthn/login/finish',
      expect.objectContaining({ session_id: 'sess-1', id: 'cred-1' })
    );
    expect(result.current.token).toBe('tok');
    expect(result.current.loading).toBe(false);
  });
});
