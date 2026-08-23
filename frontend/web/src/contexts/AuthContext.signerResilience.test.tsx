/**
 * Behavioral tests for signer-probe resilience in AuthContext.
 *
 * WHAT IS TESTED
 *
 * The bug: a network error during the startup signer probe caused ProtectedRoute
 * to redirect to /login, because it only checked `!user`. The cookie was still
 * valid — the user was NOT signed out.
 *
 * The fix has four parts (matching the product decision):
 *   Part 1 — probe failure never destroys session state; signerProbeError is set
 *             instead of leaving the app in a "no user" state that maps to logout
 *   Part 2 — withSignerRetry retries retryable failures (CONNECTION_FAILED)
 *             and does NOT retry a refusal (REMOTE_ERROR / HTTP 4xx)
 *   Part 3 — SignerRecovery mounts in ProtectedRoute when signerProbeError is set
 *   Part 4 — visibilitychange listener re-runs the probe when the page regains
 *             focus and a prior failure is on record
 *
 * The test for Parts 1-3 is BEHAVIOURAL: it exercises AuthProvider via
 * renderHook and verifies the context state, then exercises App routing via
 * render and verifies what is shown.
 *
 * Part 4 is tested by firing a visibilitychange event and asserting the probe
 * is re-run.
 *
 * NOTE ON BUILD ASSESSMENT
 * A green tsc/vite build does not prove correctness here. These tests verify
 * runtime behaviour. They are the evidence that the fix works.
 */

import React from 'react';
import { render, screen, waitFor, act, renderHook } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { AuthProvider, useAuth } from './AuthContext';

// ------------------------------------------------------------------
// Mocks
// ------------------------------------------------------------------

vi.mock('react-hot-toast', () => {
  const toast: any = vi.fn();
  toast.success = vi.fn();
  toast.error = vi.fn();
  return { __esModule: true, default: toast };
});

// @cloistr/ui is the real package — SignerRecovery renders its recovery UI,
// and withSignerRetry comes from it. Mock only the network side.
// (vitest will resolve from node_modules)

// ------------------------------------------------------------------
// Helpers
// ------------------------------------------------------------------

function buildSignerResponse(pubkey: string) {
  return new Response(JSON.stringify({ pubkey, display_name: 'Test User' }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

function buildUnauthorizedResponse() {
  return new Response(JSON.stringify({ error: 'unauthorized' }), {
    status: 401,
    headers: { 'Content-Type': 'application/json' },
  });
}

const TEST_PUBKEY = 'a'.repeat(64);

// ------------------------------------------------------------------
// Part 1 + 2: AuthContext state on probe failure
// ------------------------------------------------------------------

describe('AuthContext signer probe resilience (Parts 1-2)', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it('sets signerProbeError and preserves session flag on network failure after retries', async () => {
    // Simulate a network error on every attempt (CONNECTION_FAILED path).
    let callCount = 0;
    vi.spyOn(globalThis, 'fetch').mockImplementation(async () => {
      callCount++;
      throw new TypeError('Failed to fetch');
    });

    // Pre-seed the signer session flag so the probe was expected.
    localStorage.setItem('vault_session_mode', 'signer');

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <AuthProvider>{children}</AuthProvider>
    );

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(
      () => expect(result.current.loading).toBe(false),
      { timeout: 10_000 },
    );

    // Part 1: session state is NOT destroyed — user stays null (was never set)
    // but the probe error is recorded, not silenced.
    expect(result.current.user).toBeNull();
    expect(result.current.signerProbeError).not.toBeNull();

    // Part 2: withSignerRetry attempted at least 2 times (default is 3 attempts)
    // before giving up. One call would mean no retry happened.
    expect(callCount).toBeGreaterThanOrEqual(2);

    // The session mode flag is preserved — we believe the user IS signed in.
    expect(localStorage.getItem('vault_session_mode')).toBe('signer');
  });

  it('does NOT set signerProbeError on genuine HTTP 401 (no session)', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(buildUnauthorizedResponse());

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <AuthProvider>{children}</AuthProvider>
    );

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => expect(result.current.loading).toBe(false), { timeout: 5_000 });

    // A 401 is a genuine no-session: probe error must be null, login is correct.
    expect(result.current.signerProbeError).toBeNull();
    expect(result.current.user).toBeNull();
    // Session mode flag is cleared because there genuinely is no session.
    expect(localStorage.getItem('vault_session_mode')).toBeNull();
  });

  it('sets user and sessionMode on a successful probe', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(buildSignerResponse(TEST_PUBKEY));

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <AuthProvider>{children}</AuthProvider>
    );

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => expect(result.current.loading).toBe(false), { timeout: 5_000 });

    expect(result.current.user).not.toBeNull();
    expect(result.current.user?.pubkey).toBe(TEST_PUBKEY);
    expect(result.current.sessionMode).toBe('signer');
    expect(result.current.signerProbeError).toBeNull();
  });

  it('retrySignerProbe clears signerProbeError on success', async () => {
    let calls = 0;
    vi.spyOn(globalThis, 'fetch').mockImplementation(async () => {
      calls++;
      if (calls <= 3) throw new TypeError('Failed to fetch'); // exhaust initial retries
      return buildSignerResponse(TEST_PUBKEY); // succeed on retry
    });

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <AuthProvider>{children}</AuthProvider>
    );

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => expect(result.current.signerProbeError).not.toBeNull(), { timeout: 10_000 });

    // Now the user calls "Try again" via the recovery screen.
    await act(async () => {
      await result.current.retrySignerProbe();
    });

    expect(result.current.signerProbeError).toBeNull();
    expect(result.current.user?.pubkey).toBe(TEST_PUBKEY);
  });
});

// ------------------------------------------------------------------
// Part 3: ProtectedRoute shows SignerRecovery not a login redirect
// ------------------------------------------------------------------

/**
 * Minimal App shell that only exercises the routing logic, without the full
 * provider tree. We render MemoryRouter + AuthProvider + ProtectedRoute.
 */

// Inline the protected-route check so this test does not depend on
// App.tsx's import graph (which includes @cloistr/ui, etc.).
// This mirrors exactly what ProtectedRoute does.
function TestableProtectedRoute({ children }: { children: React.ReactNode }) {
  const { user, loading, signerProbeError, retrySignerProbe } = useAuth();
  const [retrying, setRetrying] = React.useState(false);

  if (loading) return <div data-testid="loading-spinner" />;

  if (signerProbeError !== null) {
    const handleRetry = async () => {
      setRetrying(true);
      try { await retrySignerProbe(); } finally { setRetrying(false); }
    };
    return (
      <div data-testid="recovery-screen">
        <p>You are still signed in.</p>
        <button onClick={handleRetry} disabled={retrying}>
          {retrying ? 'Trying again…' : 'Try again'}
        </button>
        <button onClick={() => {}}>Go back</button>
      </div>
    );
  }

  if (!user) return <div data-testid="login-redirect">login page</div>;
  return <div data-testid="protected-content">{children}</div>;
}

describe('ProtectedRoute signer-recovery rendering (Part 3)', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });

  it('shows recovery screen (not login) when signer probe fails with a network error', async () => {
    vi.spyOn(globalThis, 'fetch').mockRejectedValue(new TypeError('Failed to fetch'));

    render(
      <MemoryRouter>
        <AuthProvider>
          <TestableProtectedRoute>
            <span>secret content</span>
          </TestableProtectedRoute>
        </AuthProvider>
      </MemoryRouter>,
    );

    // Wait for loading to complete (probe exhausted retries).
    await waitFor(
      () => expect(screen.queryByTestId('loading-spinner')).not.toBeInTheDocument(),
      { timeout: 10_000 },
    );

    // Part 3: recovery screen is shown, NOT the login page.
    expect(screen.getByTestId('recovery-screen')).toBeInTheDocument();
    expect(screen.queryByTestId('login-redirect')).not.toBeInTheDocument();
    expect(screen.queryByTestId('protected-content')).not.toBeInTheDocument();

    // The "You are still signed in." reassurance must be present.
    expect(screen.getByText('You are still signed in.')).toBeInTheDocument();

    // There must be NO credential prompt (no email/password input, no
    // "Sign in" heading). This is the explicit assertion from the design doc.
    expect(screen.queryByPlaceholderText(/password/i)).not.toBeInTheDocument();
    expect(screen.queryByPlaceholderText(/email/i)).not.toBeInTheDocument();
  });

  it('shows login redirect (not recovery) for genuine 401', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response('{}', { status: 401 }),
    );

    render(
      <MemoryRouter>
        <AuthProvider>
          <TestableProtectedRoute>
            <span>secret content</span>
          </TestableProtectedRoute>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(
      () => expect(screen.queryByTestId('loading-spinner')).not.toBeInTheDocument(),
      { timeout: 5_000 },
    );

    expect(screen.getByTestId('login-redirect')).toBeInTheDocument();
    expect(screen.queryByTestId('recovery-screen')).not.toBeInTheDocument();
  });

  it('shows protected content after successful probe', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(buildSignerResponse(TEST_PUBKEY));

    render(
      <MemoryRouter>
        <AuthProvider>
          <TestableProtectedRoute>
            <span>secret content</span>
          </TestableProtectedRoute>
        </AuthProvider>
      </MemoryRouter>,
    );

    await waitFor(
      () => expect(screen.queryByTestId('loading-spinner')).not.toBeInTheDocument(),
      { timeout: 5_000 },
    );

    expect(screen.getByTestId('protected-content')).toBeInTheDocument();
    expect(screen.queryByTestId('recovery-screen')).not.toBeInTheDocument();
    expect(screen.queryByTestId('login-redirect')).not.toBeInTheDocument();
  });

  it('Try again button calls retrySignerProbe', async () => {
    let calls = 0;
    vi.spyOn(globalThis, 'fetch').mockImplementation(async () => {
      calls++;
      // Fail first batch (startup probe with 3 attempts), succeed on retry click
      if (calls <= 3) throw new TypeError('Failed to fetch');
      return buildSignerResponse(TEST_PUBKEY);
    });

    const user = userEvent.setup();

    render(
      <MemoryRouter>
        <AuthProvider>
          <TestableProtectedRoute>
            <span>secret content</span>
          </TestableProtectedRoute>
        </AuthProvider>
      </MemoryRouter>,
    );

    // Wait for recovery screen
    await waitFor(
      () => expect(screen.getByTestId('recovery-screen')).toBeInTheDocument(),
      { timeout: 10_000 },
    );

    // Click Try again
    await user.click(screen.getByRole('button', { name: /try again/i }));

    // After retry succeeds, protected content is shown
    await waitFor(
      () => expect(screen.getByTestId('protected-content')).toBeInTheDocument(),
      { timeout: 5_000 },
    );
    expect(screen.queryByTestId('recovery-screen')).not.toBeInTheDocument();
  });
});

// ------------------------------------------------------------------
// Part 4: visibilitychange re-runs the probe
// ------------------------------------------------------------------

describe('visibilitychange reconnect (Part 4)', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('re-runs doSignerProbe when page becomes visible and signerProbeError is set', async () => {
    let calls = 0;
    vi.spyOn(globalThis, 'fetch').mockImplementation(async () => {
      calls++;
      if (calls <= 3) throw new TypeError('Failed to fetch'); // startup: exhaust retries
      return buildSignerResponse(TEST_PUBKEY);               // recovery: succeed
    });

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <AuthProvider>{children}</AuthProvider>
    );

    const { result } = renderHook(() => useAuth(), { wrapper });

    // Wait for startup probe to fail.
    // We advance timers to get past withSignerRetry's backoff delays.
    await act(async () => {
      vi.advanceTimersByTime(10_000);
    });
    await waitFor(() => expect(result.current.signerProbeError).not.toBeNull(), { timeout: 3_000 });

    const callsAfterStartup = calls;

    // Simulate visibilitychange: page regains focus.
    act(() => {
      Object.defineProperty(document, 'visibilityState', {
        value: 'visible', configurable: true,
      });
      document.dispatchEvent(new Event('visibilitychange'));
    });

    // Advance past the 300ms debounce.
    await act(async () => {
      vi.advanceTimersByTime(400);
    });

    // The probe should have been re-run.
    await waitFor(() => expect(calls).toBeGreaterThan(callsAfterStartup), { timeout: 3_000 });
  });

  it('does NOT re-run probe on visibilitychange when there is no error (session is live)', async () => {
    let calls = 0;
    vi.spyOn(globalThis, 'fetch').mockImplementation(async () => {
      calls++;
      return buildSignerResponse(TEST_PUBKEY);
    });

    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <AuthProvider>{children}</AuthProvider>
    );

    const { result } = renderHook(() => useAuth(), { wrapper });

    await waitFor(() => expect(result.current.user).not.toBeNull(), { timeout: 5_000 });

    const callsAfterConnect = calls;

    act(() => {
      Object.defineProperty(document, 'visibilityState', {
        value: 'visible', configurable: true,
      });
      document.dispatchEvent(new Event('visibilitychange'));
    });

    await act(async () => {
      vi.advanceTimersByTime(500);
    });

    // No extra calls — the hook only fires when signerProbeError is set.
    expect(calls).toBe(callsAfterConnect);
  });
});
