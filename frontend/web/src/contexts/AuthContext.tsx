import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import axios from 'axios';
import toast from 'react-hot-toast';

interface User {
  id: string;
  email: string;
  created_at: string;
  updated_at: string;
  auth_method?: string;
  nostr_pubkey?: string;
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
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

// Configure axios defaults
axios.defaults.baseURL = '/api/v1';

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [isWebAuthnAvailable, setIsWebAuthnAvailable] = useState(false);

  useEffect(() => {
    // Check WebAuthn availability
    setIsWebAuthnAvailable(
      typeof window !== 'undefined' &&
      window.PublicKeyCredential !== undefined &&
      typeof window.PublicKeyCredential === 'function'
    );

    // Check for stored authentication on app load
    const storedToken = localStorage.getItem('vault_token');
    const storedUser = localStorage.getItem('vault_user');

    if (storedToken && storedUser) {
      try {
        setToken(storedToken);
        setUser(JSON.parse(storedUser));
        axios.defaults.headers.common['Authorization'] = `Bearer ${storedToken}`;
      } catch (error) {
        console.error('Error parsing stored user data:', error);
        localStorage.removeItem('vault_token');
        localStorage.removeItem('vault_user');
      }
    }

    setLoading(false);
  }, []);

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

      // Step 2: Create credential with authenticator
      const credential = await navigator.credentials.create({
        publicKey: options.publicKey,
      }) as PublicKeyCredential;

      if (!credential) {
        throw new Error('No credential returned');
      }

      const response = credential.response as AuthenticatorAttestationResponse;

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

      toast.success('Passkey registered successfully!');
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
    try {
      if (token) {
        await axios.post('/auth/logout');
      }
    } catch (error) {
      console.error('Logout error:', error);
    } finally {
      setToken(null);
      setUser(null);
      
      localStorage.removeItem('vault_token');
      localStorage.removeItem('vault_user');
      
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