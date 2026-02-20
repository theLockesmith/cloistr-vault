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

  useEffect(() => {
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