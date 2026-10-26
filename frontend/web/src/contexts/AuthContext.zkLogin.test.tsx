/**
 * Email accounts must never send the master password to the server. The
 * client sends an auth key derived from it instead; only an account created
 * before that change sends the password, once, to be upgraded.
 */
import React from 'react';
import { renderHook, waitFor, act } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import axios from 'axios';
import { AuthProvider, useAuth } from './AuthContext';
import { deriveAuthKey } from '../crypto/vault';

vi.mock('react-hot-toast', () => {
  const toast: any = vi.fn();
  toast.success = vi.fn();
  toast.error = vi.fn();
  return { __esModule: true, default: toast };
});

const EMAIL = 'alice@example.com';
const PASSWORD = 'correct horse battery staple';

function wrapper({ children }: { children: React.ReactNode }) {
  return (
    <MemoryRouter>
      <AuthProvider>{children}</AuthProvider>
    </MemoryRouter>
  );
}

function ok(data: unknown) {
  return Promise.resolve({ data, status: 200 });
}

function httpError(status: number, data: unknown) {
  return Promise.reject(Object.assign(new Error(`HTTP ${status}`), { response: { status, data } }));
}

const session = { token: 't', user: { id: 'u', email: EMAIL } };

async function renderAuth() {
  // No signer session, so the provider settles on "signed out".
  vi.spyOn(globalThis, 'fetch').mockResolvedValue(
    new Response('{}', { status: 401, headers: { 'Content-Type': 'application/json' } })
  );
  const { result } = renderHook(() => useAuth(), { wrapper });
  await waitFor(() => expect(result.current.loading).toBe(false));
  return result;
}

function bodies(post: { mock: { calls: unknown[][] } }): Record<string, unknown>[] {
  return post.mock.calls.map((c) => c[1] as Record<string, unknown>);
}

function expectNoPassword(body: Record<string, unknown>) {
  expect(body).not.toHaveProperty('password');
  expect(JSON.stringify(body)).not.toContain(PASSWORD);
}

describe('email sign-in never sends the master password', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it('register sends the auth key and not the password', async () => {
    const result = await renderAuth();
    const post = vi.spyOn(axios, 'post').mockImplementation(() => ok({}));

    await act(() => result.current.register(EMAIL, PASSWORD, 'envelope'));

    const [body] = bodies(post);
    expectNoPassword(body);
    expect(body.auth_key).toBe(await deriveAuthKey(EMAIL, PASSWORD));
    expect(body.vault_data).toBe('envelope');
  }, 30_000);

  it('login sends the auth key and not the password', async () => {
    const result = await renderAuth();
    const post = vi.spyOn(axios, 'post').mockImplementation(() => ok(session));

    await act(() => result.current.login(EMAIL, PASSWORD));

    expect(post).toHaveBeenCalledTimes(1);
    const [body] = bodies(post);
    expectNoPassword(body);
    expect(body.auth_key).toBe(await deriveAuthKey(EMAIL, PASSWORD));
  }, 30_000);

  it('a legacy account sends the password once to upgrade, and never again', async () => {
    const result = await renderAuth();
    const post = vi
      .spyOn(axios, 'post')
      .mockImplementationOnce(() => httpError(409, { code: 'AUTH_UPGRADE_REQUIRED' }))
      .mockImplementation(() => ok(session));

    await act(() => result.current.login(EMAIL, PASSWORD));

    const [first, upgrade] = bodies(post);
    expectNoPassword(first);
    expect(upgrade.password).toBe(PASSWORD);
    expect(upgrade.auth_key).toBe(first.auth_key);

    // The server has swapped its verifier; the next sign-in is auth key only.
    post.mockClear();
    await act(() => result.current.login(EMAIL, PASSWORD));
    expect(post).toHaveBeenCalledTimes(1);
    expectNoPassword(bodies(post)[0]);
  }, 60_000);

  it('a wrong password is not retried with the password', async () => {
    const result = await renderAuth();
    const post = vi
      .spyOn(axios, 'post')
      .mockImplementation(() => httpError(401, { code: 'AUTH_INVALID', message: 'Invalid email or password' }));

    await act(async () => {
      await expect(result.current.login(EMAIL, PASSWORD)).rejects.toThrow();
    });

    expect(post).toHaveBeenCalledTimes(1);
    expectNoPassword(bodies(post)[0]);
  }, 30_000);
});
