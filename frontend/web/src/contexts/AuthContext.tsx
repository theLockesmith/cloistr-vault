import React, { createContext, useContext, useState, useEffect, useRef, ReactNode } from 'react';
import axios from 'axios';
import toast from 'react-hot-toast';
import { withSignerRetry, isRetryableSignerError } from '@cloistr/ui';

interface User {
  id?: string;
  email?: string;
  created_at?: string;
  updated_at?: string;
  auth_method?: string;
  nostr_pubkey?: string;
  pubkey?: string;
  nip05_address?: string;
  lightning_address?: string;
  display_name?: string;
}

interface NIP05LookupResult {
  nip05_address: string;
  pubkey: string;
  relays: string[];
}

interface LightningChallenge {
  k1: string;
  lnurl: string;
  expiresAt: string;
}

interface WebAuthnCredential {
  id: string;
  credential_id: string;
  name: string;
  created_at: string;
  last_used_at?: string;
  backup_eligible: boolean;
  backup_state: boolean;
}

interface AuthContextType {
  user: User | null;
  token: string | null;
  login: (email: string, password: string) => Promise<void>;
  loginWithNostr: (publicKey: string, signature: string, challenge: string) => Promise<void>;
  getLightningChallenge: (lightningAddress: string) => Promise<LightningChallenge>;
  loginWithLightning: (k1: string, signature: string, linkingKey: string) => Promise<void>;
  register: (email: string, password: string, vaultData: string) => Promise<void>;
  registerWithNostr: (publicKey: string, vaultData: string) => Promise<void>;
  verifyNIP05: (nip05Address: string) => Promise<void>;
  lookupNIP05: (nip05Address: string) => Promise<NIP05LookupResult>;
  loginWithWebAuthn: (email: string) => Promise<void>;
  loginWithWebAuthnDiscoverable: () => Promise<void>;
  registerWebAuthnCredential: (name: string) => Promise<WebAuthnCredential>;
  listWebAuthnCredentials: () => Promise<WebAuthnCredential[]>;
  deleteWebAuthnCredential: (credentialId: string) => Promise<void>;
  updateWebAuthnCredential: (credentialId: string, name: string) => Promise<void>;
  isWebAuthnAvailable: boolean;
  logout: () => void;
  loading: boolean;
  sessionMode: 'vault' | 'signer' | null;
  /**
   * Set when the startup signer probe failed due to a transient network error.
   * The session may still be valid — the signer was temporarily unreachable.
   * Never set for a genuine absence of session (HTTP 4xx = no session = null).
   */
  signerProbeError: unknown | null;
  /** Re-runs the signer probe. Call from a recovery screen, not silently. */
  retrySignerProbe: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

// Configure axios defaults
axios.defaults.baseURL = '/api/v1';
// Send cookies cross-origin so the .cloistr.xyz signer session cookie
// reaches vault.cloistr.xyz automatically on every request.
axios.defaults.withCredentials = true;

/**
 * Build a fetch error with a code that classifySignerError (from @cloistr/ui)
 * recognises as retryable. The signer probe uses HTTP, not NIP-46, but the
 * same retry classification applies: a network exception means "couldn't reach
 * the signer", which is retryable and must not be treated as "no session".
 */
function makeRetryableError(cause: unknown): Error & { code: string } {
  const err = new Error(
    cause instanceof Error ? cause.message : 'Signer unreachable',
  ) as Error & { code: string };
  err.code = 'CONNECTION_FAILED';
  err.cause = cause;
  return err;
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [isWebAuthnAvailable, setIsWebAuthnAvailable] = useState(false);
  const [sessionMode, setSessionMode] = useState<'vault' | 'signer' | null>(null);
  /**
   * Tracks a transient signer-probe failure.
   *
   * WHY THIS EXISTS — and why it is NOT a logout signal:
   *
   * session            = who you are (backend JWT + SSO cookie)
   * signer reachability = can we reach signer.cloistr.xyz right now
   *
   * Before this change, a network error during the startup probe cleared the
   * session flag and let ProtectedRoute redirect to /login. That is wrong: the
   * cookie is still valid; only the HTTP request timed out. Users on mobile
   * see this every time they switch apps and back: one hiccup, and they are
   * at a credential prompt for a session that was never actually invalid.
   *
   * signerProbeError is only set for NETWORK errors (fetch throws). An HTTP
   * 401/403 is a genuine "no session" and leaves this null — login IS correct.
   */
  const [signerProbeError, setSignerProbeError] = useState<unknown | null>(null);

  // Keep probe-error state in a ref so the visibilitychange handler always
  // reads the current value without being torn down and re-registered on every
  // state change (the same pattern useRelayReconnect uses for authState).
  const signerProbeErrorRef = useRef<unknown | null>(null);
  useEffect(() => {
    signerProbeErrorRef.current = signerProbeError;
  }, [signerProbeError]);

  useEffect(() => {
    // Check WebAuthn availability
    setIsWebAuthnAvailable(
      typeof window !== 'undefined' &&
      window.PublicKeyCredential !== undefined &&
      typeof window.PublicKeyCredential === 'function'
    );

    void (async () => {
      // 1. Vault token takes precedence -- restore immediately.
      const storedToken = localStorage.getItem('vault_token');
      const storedUser = localStorage.getItem('vault_user');

      if (storedToken && storedUser) {
        try {
          setToken(storedToken);
          setUser(JSON.parse(storedUser));
          axios.defaults.headers.common['Authorization'] = `Bearer ${storedToken}`;
          setSessionMode('vault');
          setLoading(false);
          return;
        } catch (error) {
          console.error('Error parsing stored user data:', error);
          localStorage.removeItem('vault_token');
          localStorage.removeItem('vault_user');
        }
      }

      // 2. No vault token -- probe the signer for a shared session.
      //    The .cloistr.xyz auth_token cookie rides automatically because
      //    withCredentials = true is set globally above.
      //    Keep loading=true through all retry attempts. ProtectedRoute should
      //    show the spinner, not the recovery screen, while retries are in
      //    flight. Only after doSignerProbe() finishes (success, no-session,
      //    or retries-exhausted) does the loading state resolve.
      await doSignerProbe();
      setLoading(false);
    })();
  }, []);

  /**
   * Part 4 — reconnect on visibilitychange.
   *
   * When the OS backgrounds the page (app-switcher, file picker, screen lock)
   * it can kill HTTP connections in flight. If that happened to the startup
   * probe, signerProbeError is set and the recovery screen is showing. When the
   * user flips back to the tab, re-run the probe silently so a mere background
   * trip does not keep them on the recovery screen indefinitely.
   *
   * Registered once; the handler reads current state via refs so it never needs
   * to be torn down and re-added on state changes (avoids the listen-gap during
   * rapid transitions that useRelayReconnect also avoids with the same pattern).
   */
  useEffect(() => {
    const DEBOUNCE_MS = 300;
    let timer: ReturnType<typeof setTimeout> | null = null;

    const scheduleReprobe = () => {
      if (timer !== null) clearTimeout(timer);
      timer = setTimeout(() => {
        timer = null;
        // Only re-probe when a transient failure is on record. If the user is
        // already signed in (sessionMode set) or at the deliberate login page
        // (no error, no session), there is nothing to recover.
        if (signerProbeErrorRef.current !== null) {
          void doSignerProbe();
        }
      }, DEBOUNCE_MS);
    };

    const onVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        scheduleReprobe();
      }
    };

    const onOnline = () => {
      scheduleReprobe();
    };

    document.addEventListener('visibilitychange', onVisibilityChange);
    window.addEventListener('online', onOnline);

    return () => {
      document.removeEventListener('visibilitychange', onVisibilityChange);
      window.removeEventListener('online', onOnline);
      if (timer !== null) clearTimeout(timer);
    };
  }, []); // empty deps: handler reads current state via ref

  /**
   * Probe signer.cloistr.xyz for a shared session using withSignerRetry.
   *
   * THREE OUTCOMES:
   *  - success:    session found, setUser + setSessionMode('signer')
   *  - no-session: HTTP 4xx, fall through — user stays null, login is correct
   *  - retryable exhausted: network error after 3 attempts, setSignerProbeError
   *
   * This function is exported as retrySignerProbe so the recovery screen can
   * call it when the user clicks "Try again".
   */
  const doSignerProbe = async (): Promise<void> => {
    setSignerProbeError(null);
    try {
      const signerUser = await withSignerRetry<User | null>(
        async () => {
          let resp: Response;
          try {
            resp = await fetch('https://signer.cloistr.xyz/api/v1/users/me', {
              credentials: 'include',
            });
          } catch (fetchErr) {
            // Network-level failure: socket closed, DNS failed, offline.
            // Promote to a retryable error so withSignerRetry will back off
            // and try again before giving up.
            throw makeRetryableError(fetchErr);
          }

          if (resp.ok) {
            const data = await resp.json();
            if (data?.pubkey) {
              return {
                pubkey: data.pubkey,
                nostr_pubkey: data.pubkey,
                display_name: data.display_name ?? data.name ?? undefined,
                nip05_address: data.nip05 ?? undefined,
              } as User;
            }
          }
          // HTTP 4xx/5xx: probe reached the server but there is no valid
          // session. This is NOT retryable — the server made a decision.
          // Return null to signal "no session, fall through to login".
          return null;
        },
        { attempts: 3, baseDelayMs: 300, maxDelayMs: 4000 },
      );

      if (signerUser !== null) {
        setUser(signerUser);
        setSessionMode('signer');
        localStorage.setItem('vault_session_mode', 'signer');
      } else {
        // Genuine no-session from the server. Clearing this flag is correct:
        // the user is not signed in via the signer.
        localStorage.removeItem('vault_session_mode');
        // signerProbeError stays null — the user should see the login page.
      }
    } catch (err) {
      // withSignerRetry rethrows after all attempts are exhausted.
      // The error carries code='CONNECTION_FAILED', so isRetryableSignerError
      // is true — this was a transient failure, NOT a session invalidation.
      if (isRetryableSignerError(err)) {
        // Keep vault_session_mode if it was set: we believe the user still has
        // a valid signer session, we just couldn't reach it. Clearing the flag
        // here would send them to the login page — exactly the bug we are fixing.
        setSignerProbeError(err);
      } else {
        // Unexpected terminal error during the probe. Treat as no-session.
        localStorage.removeItem('vault_session_mode');
      }
      console.debug('Signer probe failed:', err);
    }
  };

  const retrySignerProbe = async (): Promise<void> => {
    await doSignerProbe();
  };

  const login = async (email: string, password: string) => {
    try {
      setLoading(true);
      const response = await axios.post('/auth/login', {
        method: 'email',
        email,
        password,
      });

      const { token: newToken, user: newUser } = response.data;

      setToken(newToken);
      setUser(newUser);

      localStorage.setItem('vault_token', newToken);
      localStorage.setItem('vault_user', JSON.stringify(newUser));

      axios.defaults.headers.common['Authorization'] = `Bearer ${newToken}`;

      toast.success('Successfully logged in!');
    } catch (error: any) {
      const message = error.response?.data?.error || 'Login failed';
      toast.error(message);
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const loginWithNostr = async (publicKey: string, signature: string, challenge: string) => {
    try {
      setLoading(true);
      const response = await axios.post('/auth/login', {
        method: 'nostr',
        nostr_pubkey: publicKey,
        signature,
        challenge,
      });

      const { token: newToken, user: newUser } = response.data;

      setToken(newToken);
      setUser(newUser);

      localStorage.setItem('vault_token', newToken);
      localStorage.setItem('vault_user', JSON.stringify(newUser));

      axios.defaults.headers.common['Authorization'] = `Bearer ${newToken}`;

      toast.success('Successfully logged in with Nostr!');
    } catch (error: any) {
      const message = error.response?.data?.error || 'Nostr login failed';
      toast.error(message);
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const getLightningChallenge = async (lightningAddress: string): Promise<LightningChallenge> => {
    try {
      setLoading(true);
      const response = await axios.post('/auth/lightning/challenge', {
        lightning_address: lightningAddress,
      });

      return {
        k1: response.data.k1,
        lnurl: response.data.lnurl,
        expiresAt: response.data.expires_at,
      };
    } catch (error: any) {
      const message = error.response?.data?.error || 'Failed to get Lightning challenge';
      toast.error(message);
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const loginWithLightning = async (k1: string, signature: string, linkingKey: string) => {
    try {
      setLoading(true);
      const response = await axios.post('/auth/login', {
        method: 'lightning_address',
        k1,
        signature,
        linking_key: linkingKey,
      });

      const { token: newToken, user: newUser } = response.data;

      setToken(newToken);
      setUser(newUser);

      localStorage.setItem('vault_token', newToken);
      localStorage.setItem('vault_user', JSON.stringify(newUser));

      axios.defaults.headers.common['Authorization'] = `Bearer ${newToken}`;

      toast.success('Successfully logged in with Lightning!');
    } catch (error: any) {
      const message = error.response?.data?.error || 'Lightning login failed';
      toast.error(message);
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const register = async (email: string, password: string, vaultData: string) => {
    try {
      setLoading(true);
      await axios.post('/auth/register', {
        method: 'email',
        email,
        password,
        vault_data: vaultData,
      });

      toast.success('Account created successfully! Please log in.');
    } catch (error: any) {
      const message = error.response?.data?.error || 'Registration failed';
      toast.error(message);
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const registerWithNostr = async (publicKey: string, vaultData: string) => {
    try {
      setLoading(true);
      await axios.post('/auth/register', {
        method: 'nostr',
        nostr_pubkey: publicKey,
        vault_data: vaultData,
      });

      toast.success('Account created successfully with Nostr! Please log in.');
    } catch (error: any) {
      const message = error.response?.data?.error || 'Nostr registration failed';
      toast.error(message);
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const verifyNIP05 = async (nip05Address: string) => {
    try {
      setLoading(true);
      await axios.post('/nip05/verify', {
        nip05_address: nip05Address,
      });

      // Update local user state with verified NIP-05
      if (user) {
        const updatedUser = { ...user, nip05_address: nip05Address };
        setUser(updatedUser);
        localStorage.setItem('vault_user', JSON.stringify(updatedUser));
      }

      toast.success('NIP-05 address verified successfully!');
    } catch (error: any) {
      const message = error.response?.data?.error || 'NIP-05 verification failed';
      toast.error(message);
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const lookupNIP05 = async (nip05Address: string): Promise<NIP05LookupResult> => {
    try {
      setLoading(true);
      const response = await axios.get(`/nip05/lookup?address=${encodeURIComponent(nip05Address)}`);
      return response.data;
    } catch (error: any) {
      const message = error.response?.data?.error || 'NIP-05 lookup failed';
      toast.error(message);
      throw error;
    } finally {
      setLoading(false);
    }
  };

  // WebAuthn helper functions
  const arrayBufferToBase64Url = (buffer: ArrayBuffer): string => {
    const bytes = new Uint8Array(buffer);
    let binary = '';
    bytes.forEach((b) => binary += String.fromCharCode(b));
    return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
  };

  const base64UrlToArrayBuffer = (base64url: string): ArrayBuffer => {
    const base64 = base64url.replace(/-/g, '+').replace(/_/g, '/');
    const padding = '='.repeat((4 - base64.length % 4) % 4);
    const binary = atob(base64 + padding);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
      bytes[i] = binary.charCodeAt(i);
    }
    return bytes.buffer;
  };

  const loginWithWebAuthn = async (email: string) => {
    if (!isWebAuthnAvailable) {
      throw new Error('WebAuthn is not supported in this browser');
    }

    try {
      setLoading(true);

      // Step 1: Get challenge from server
      const beginResponse = await axios.post('/auth/webauthn/login/begin', { email });
      const options = beginResponse.data;

      // Convert base64url strings to ArrayBuffers
      options.publicKey.challenge = base64UrlToArrayBuffer(options.publicKey.challenge);
      if (options.publicKey.allowCredentials) {
        options.publicKey.allowCredentials = options.publicKey.allowCredentials.map((cred: any) => ({
          ...cred,
          id: base64UrlToArrayBuffer(cred.id),
        }));
      }

      // Step 2: Get credential from authenticator
      const credential = await navigator.credentials.get({
        publicKey: options.publicKey,
      }) as PublicKeyCredential;

      if (!credential) {
        throw new Error('No credential returned');
      }

      const response = credential.response as AuthenticatorAssertionResponse;

      // Step 3: Send credential to server
      const finishResponse = await axios.post('/auth/webauthn/login/finish', {
        email,
        id: credential.id,
        rawId: arrayBufferToBase64Url(credential.rawId),
        type: credential.type,
        response: {
          authenticatorData: arrayBufferToBase64Url(response.authenticatorData),
          clientDataJSON: arrayBufferToBase64Url(response.clientDataJSON),
          signature: arrayBufferToBase64Url(response.signature),
          userHandle: response.userHandle ? arrayBufferToBase64Url(response.userHandle) : null,
        },
      });

      const { token: newToken, user: newUser } = finishResponse.data;

      setToken(newToken);
      setUser(newUser);

      localStorage.setItem('vault_token', newToken);
      localStorage.setItem('vault_user', JSON.stringify(newUser));

      axios.defaults.headers.common['Authorization'] = `Bearer ${newToken}`;

      toast.success('Successfully logged in with passkey!');
    } catch (error: any) {
      const message = error.response?.data?.error || error.message || 'Passkey login failed';
      toast.error(message);
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const loginWithWebAuthnDiscoverable = async () => {
    if (!isWebAuthnAvailable) {
      throw new Error('WebAuthn is not supported in this browser');
    }

    try {
      setLoading(true);

      // Step 1: Get challenge from server (no email needed)
      const beginResponse = await axios.post('/auth/webauthn/login/begin/discoverable');
      const { options, session_id } = beginResponse.data;

      // Convert base64url strings to ArrayBuffers
      options.publicKey.challenge = base64UrlToArrayBuffer(options.publicKey.challenge);

      // Step 2: Get credential from authenticator (browser will show available passkeys)
      const credential = await navigator.credentials.get({
        publicKey: options.publicKey,
        mediation: 'optional',
      }) as PublicKeyCredential;

      if (!credential) {
        throw new Error('No credential returned');
      }

      const response = credential.response as AuthenticatorAssertionResponse;

      // Step 3: Send credential to server
      const finishResponse = await axios.post('/auth/webauthn/login/finish', {
        session_id,
        id: credential.id,
        rawId: arrayBufferToBase64Url(credential.rawId),
        type: credential.type,
        response: {
          authenticatorData: arrayBufferToBase64Url(response.authenticatorData),
          clientDataJSON: arrayBufferToBase64Url(response.clientDataJSON),
          signature: arrayBufferToBase64Url(response.signature),
          userHandle: response.userHandle ? arrayBufferToBase64Url(response.userHandle) : null,
        },
      });

      const { token: newToken, user: newUser } = finishResponse.data;

      setToken(newToken);
      setUser(newUser);

      localStorage.setItem('vault_token', newToken);
      localStorage.setItem('vault_user', JSON.stringify(newUser));

      axios.defaults.headers.common['Authorization'] = `Bearer ${newToken}`;

      toast.success('Successfully logged in with passkey!');
    } catch (error: any) {
      const message = error.response?.data?.error || error.message || 'Passkey login failed';
      toast.error(message);
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const registerWebAuthnCredential = async (name: string): Promise<WebAuthnCredential> => {
    if (!isWebAuthnAvailable) {
      throw new Error('WebAuthn is not supported in this browser');
    }

    try {
      setLoading(true);

      // Step 1: Get registration options from server
      const beginResponse = await axios.post('/user/webauthn/register/begin');
      const options = beginResponse.data;

      // Convert base64url strings to ArrayBuffers
      options.publicKey.challenge = base64UrlToArrayBuffer(options.publicKey.challenge);
      options.publicKey.user.id = base64UrlToArrayBuffer(options.publicKey.user.id);
      if (options.publicKey.excludeCredentials) {
        options.publicKey.excludeCredentials = options.publicKey.excludeCredentials.map((cred: any) => ({
          ...cred,
          id: base64UrlToArrayBuffer(cred.id),
        }));
      }

      // Ask for the PRF extension so this credential can unlock the vault, not
      // just prove identity. The server requests it too; setting it here as
      // well means a client talking to an older replica mid-rollout still gets
      // a PRF-capable passkey. PRF must be requested at creation — an
      // authenticator will not retrofit hmac-secret onto an existing
      // credential.
      options.publicKey.extensions = {
        ...(options.publicKey.extensions || {}),
        prf: {},
      };

      // Step 2: Create credential with authenticator
      const credential = await navigator.credentials.create({
        publicKey: options.publicKey,
      }) as PublicKeyCredential;

      if (!credential) {
        throw new Error('No credential returned');
      }

      const response = credential.response as AuthenticatorAttestationResponse;
      // `enabled` reports whether the authenticator will evaluate the PRF for
      // this credential. Absent or false means the passkey signs in fine but
      // can never unlock the vault, which is worth saying at registration
      // rather than letting the user discover it at the lock screen.
      const prfEnabled =
        (credential.getClientExtensionResults() as { prf?: { enabled?: boolean } }).prf?.enabled ===
        true;

      // Step 3: Send credential to server
      const finishResponse = await axios.post(`/user/webauthn/register/finish?name=${encodeURIComponent(name)}`, {
        id: credential.id,
        rawId: arrayBufferToBase64Url(credential.rawId),
        type: credential.type,
        response: {
          attestationObject: arrayBufferToBase64Url(response.attestationObject),
          clientDataJSON: arrayBufferToBase64Url(response.clientDataJSON),
        },
      });

      if (prfEnabled) {
        toast.success('Passkey registered — you can enable vault unlock for it in Settings');
      } else {
        toast.success('Passkey registered successfully!');
        toast('This passkey can sign you in but cannot unlock your vault', { icon: 'ℹ️' });
      }
      return finishResponse.data.credential;
    } catch (error: any) {
      const message = error.response?.data?.error || error.message || 'Failed to register passkey';
      toast.error(message);
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const listWebAuthnCredentials = async (): Promise<WebAuthnCredential[]> => {
    try {
      const response = await axios.get('/user/webauthn/credentials');
      return response.data.credentials || [];
    } catch (error: any) {
      const message = error.response?.data?.error || 'Failed to list passkeys';
      toast.error(message);
      throw error;
    }
  };

  const deleteWebAuthnCredential = async (credentialId: string) => {
    try {
      await axios.delete(`/user/webauthn/credentials/${encodeURIComponent(credentialId)}`);
      toast.success('Passkey removed successfully');
    } catch (error: any) {
      const message = error.response?.data?.error || 'Failed to remove passkey';
      toast.error(message);
      throw error;
    }
  };

  const updateWebAuthnCredential = async (credentialId: string, name: string) => {
    try {
      await axios.put(`/user/webauthn/credentials/${encodeURIComponent(credentialId)}`, { name });
      toast.success('Passkey renamed successfully');
    } catch (error: any) {
      const message = error.response?.data?.error || 'Failed to rename passkey';
      toast.error(message);
      throw error;
    }
  };

  const logout = async () => {
    if (sessionMode === 'signer') {
      // Signer-session logout: clear local state and flag only.
      // The .cloistr.xyz cookie remains valid -- vault just forgets the session.
      setUser(null);
      setSessionMode(null);
      setSignerProbeError(null);
      localStorage.removeItem('vault_session_mode');
      toast.success('Logged out successfully');
      return;
    }

    try {
      if (token) {
        await axios.post('/auth/logout');
      }
    } catch (error) {
      console.error('Logout error:', error);
    } finally {
      setToken(null);
      setUser(null);
      setSessionMode(null);
      setSignerProbeError(null);

      localStorage.removeItem('vault_token');
      localStorage.removeItem('vault_user');
      localStorage.removeItem('vault_session_mode');

      delete axios.defaults.headers.common['Authorization'];

      toast.success('Logged out successfully');
    }
  };

  const value = {
    user,
    token,
    login,
    loginWithNostr,
    getLightningChallenge,
    loginWithLightning,
    register,
    registerWithNostr,
    verifyNIP05,
    lookupNIP05,
    loginWithWebAuthn,
    loginWithWebAuthnDiscoverable,
    registerWebAuthnCredential,
    listWebAuthnCredentials,
    deleteWebAuthnCredential,
    updateWebAuthnCredential,
    isWebAuthnAvailable,
    logout,
    loading,
    sessionMode,
    signerProbeError,
    retrySignerProbe,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const context = useContext(AuthContext);
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
}
