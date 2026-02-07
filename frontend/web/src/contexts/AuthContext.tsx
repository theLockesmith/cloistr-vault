import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import axios from 'axios';
import toast from 'react-hot-toast';

interface User {
  id: string;
  email: string;
  created_at: string;
  updated_at: string;
}

interface AuthContextType {
  user: User | null;
  token: string | null;
  login: (email: string, password: string) => Promise<void>;
  loginWithNostr: (publicKey: string, signature: string, challenge: string) => Promise<void>;
  register: (email: string, password: string, vaultData: string) => Promise<void>;
  registerWithNostr: (publicKey: string, vaultData: string) => Promise<void>;
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
    register,
    registerWithNostr,
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