/**
 * The login page offers saved passkeys in the email field's autofill. It keeps
 * one conditional WebAuthn request pending while it is open, renews it before
 * the server's five-minute challenge expires, and cancels it on leaving.
 */
import React from 'react';
import { render, screen, act, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Login from './Login';

const startPasskeyAutofill = vi.fn();
const loginWithWebAuthnDiscoverable = vi.fn();

vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({
    login: vi.fn(),
    loginWithNostr: vi.fn(),
    getLightningChallenge: vi.fn(),
    loginWithLightning: vi.fn(),
    loginWithWebAuthn: vi.fn(),
    loginWithWebAuthnDiscoverable,
    startPasskeyAutofill,
    isWebAuthnAvailable: true,
    loading: false,
  }),
}));

vi.mock('@cloistr/ui', () => ({ Header: () => null }));
vi.mock('@cloistr/collab-common/config', () => ({
  getServiceConfig: () => ({ signerUrl: 'https://signer.example' }),
}));

/** Resolves with the signal's abort reason, like a real conditional request. */
function pendingUntilAborted(signal: AbortSignal) {
  return new Promise<string>((resolve) => {
    signal.addEventListener('abort', () => resolve('aborted'));
  });
}

function renderLogin() {
  return render(
    <MemoryRouter>
      <Login />
    </MemoryRouter>
  );
}

describe('Login passkey autofill', () => {
  beforeEach(() => {
    startPasskeyAutofill.mockReset();
    loginWithWebAuthnDiscoverable.mockReset();
    startPasskeyAutofill.mockImplementation(pendingUntilAborted);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('marks the email field for passkey autofill', () => {
    renderLogin();
    const email = screen.getByPlaceholderText('Enter your email');
    expect(email.getAttribute('autocomplete')).toBe('username webauthn');
  });

  it('starts one autofill request on open and cancels it on leaving', async () => {
    const { unmount } = renderLogin();
    await act(async () => {});
    expect(startPasskeyAutofill).toHaveBeenCalledTimes(1);
    const signal: AbortSignal = startPasskeyAutofill.mock.calls[0][0];
    expect(signal.aborted).toBe(false);

    unmount();
    expect(signal.aborted).toBe(true);
  });

  it('renews the request before the server challenge expires', async () => {
    vi.useFakeTimers();
    renderLogin();
    await act(async () => {});
    expect(startPasskeyAutofill).toHaveBeenCalledTimes(1);
    const first: AbortSignal = startPasskeyAutofill.mock.calls[0][0];

    await act(async () => {
      vi.advanceTimersByTime(4 * 60 * 1000);
    });

    expect(first.aborted).toBe(true);
    expect(startPasskeyAutofill).toHaveBeenCalledTimes(2);
    expect(startPasskeyAutofill.mock.calls[1][0].aborted).toBe(false);
  });

  it('stops when the request fails rather than retrying in a loop', async () => {
    startPasskeyAutofill.mockResolvedValue('failed');
    renderLogin();
    await act(async () => {});
    await act(async () => {});
    expect(startPasskeyAutofill).toHaveBeenCalledTimes(1);
  });

  it('cancels autofill before the Sign in with Passkey button, and re-arms if it fails', async () => {
    let fail!: () => void;
    loginWithWebAuthnDiscoverable.mockReturnValue(
      new Promise<void>((_, reject) => (fail = () => reject(new Error('NotAllowedError')))),
    );
    renderLogin();
    await act(async () => {});
    const first: AbortSignal = startPasskeyAutofill.mock.calls[0][0];

    fireEvent.click(screen.getByRole('button', { name: /passkey/i }));
    fireEvent.click(screen.getByRole('button', { name: /^sign in with passkey$/i }));
    expect(first.aborted).toBe(true);
    expect(loginWithWebAuthnDiscoverable).toHaveBeenCalledTimes(1);
    expect(startPasskeyAutofill).toHaveBeenCalledTimes(1);

    await act(async () => {
      fail();
    });
    expect(startPasskeyAutofill).toHaveBeenCalledTimes(2);
  });

  it('does not re-arm after the passkey button signs in', async () => {
    loginWithWebAuthnDiscoverable.mockResolvedValue(undefined);
    renderLogin();
    await act(async () => {});

    fireEvent.click(screen.getByRole('button', { name: /passkey/i }));
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /^sign in with passkey$/i }));
    });
    expect(loginWithWebAuthnDiscoverable).toHaveBeenCalledTimes(1);
    expect(startPasskeyAutofill).toHaveBeenCalledTimes(1);
  });
});
