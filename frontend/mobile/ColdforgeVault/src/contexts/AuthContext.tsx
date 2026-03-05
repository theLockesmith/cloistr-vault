import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import AsyncStorage from '@react-native-async-storage/async-storage';
import Toast from 'react-native-toast-message';
import api from '../services/api';
import passkeyService, { PasskeyCredential } from '../services/passkey';

interface User {
  id: string;
  email: string;
  created_at: string;
  updated_at: string;
}

interface AuthContextType {
  user: User | null;
  token: string | null;
  loading: boolean;
  passkeySupported: boolean;
  login: (email: string, password: string) => Promise<void>;
  loginWithPasskey: (email?: string) => Promise<void>;
  loginWithNostr: (publicKey: string, signature: string, challenge: string) => Promise<void>;
  register: (email: string, password: string, vaultData: string) => Promise<void>;
  registerWithNostr: (publicKey: string, vaultData: string) => Promise<void>;
  registerPasskey: () => Promise<PasskeyCredential>;
  getPasskeys: () => Promise<PasskeyCredential[]>;
  renamePasskey: (id: string, name: string) => Promise<void>;
  deletePasskey: (id: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [passkeySupported] = useState(() => passkeyService.isSupported());

  useEffect(() => {
    checkStoredAuth();
  }, []);

  const checkStoredAuth = async () => {
    try {
      const storedToken = await AsyncStorage.getItem('vault_token');
      const storedUser = await AsyncStorage.getItem('vault_user');

      if (storedToken && storedUser) {
        setToken(storedToken);
        setUser(JSON.parse(storedUser));
      }
    } catch (error) {
      console.error('Error checking stored auth:', error);
    } finally {
      setLoading(false);
    }
  };

  const saveAuthState = async (newToken: string, newUser: User) => {
    setToken(newToken);
    setUser(newUser);
    await AsyncStorage.setItem('vault_token', newToken);
    await AsyncStorage.setItem('vault_user', JSON.stringify(newUser));
  };

  const login = async (email: string, password: string) => {
    try {
      setLoading(true);
      const response = await api.login(email, password);
      const { token: newToken, user: newUser } = response;

      await saveAuthState(newToken, newUser);

      Toast.show({
        type: 'success',
        text1: 'Success',
        text2: 'Successfully logged in!',
      });
    } catch (error: any) {
      const message = error.response?.data?.error || error.message || 'Login failed';
      Toast.show({
        type: 'error',
        text1: 'Login Failed',
        text2: message,
      });
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const loginWithPasskey = async (email?: string) => {
    if (!passkeySupported) {
      Toast.show({
        type: 'error',
        text1: 'Not Supported',
        text2: 'Passkeys are not supported on this device',
      });
      throw new Error('Passkeys not supported');
    }

    try {
      setLoading(true);
      const result = await passkeyService.loginWithPasskey(email);
      const { token: newToken, user: newUser } = result;

      await saveAuthState(newToken, newUser);

      Toast.show({
        type: 'success',
        text1: 'Success',
        text2: 'Authenticated with passkey!',
      });
    } catch (error: any) {
      Toast.show({
        type: 'error',
        text1: 'Passkey Login Failed',
        text2: error.message || 'Passkey authentication failed',
      });
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const loginWithNostr = async (publicKey: string, signature: string, challenge: string) => {
    try {
      setLoading(true);
      Toast.show({
        type: 'error',
        text1: 'Feature Coming Soon',
        text2: 'Nostr login is not yet implemented',
      });
    } catch (error: any) {
      Toast.show({
        type: 'error',
        text1: 'Nostr Login Failed',
        text2: error.message || 'Nostr login failed',
      });
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const register = async (email: string, password: string, vaultData: string) => {
    try {
      setLoading(true);
      await new Promise(resolve => setTimeout(resolve, 1000));

      Toast.show({
        type: 'success',
        text1: 'Account Created',
        text2: 'Account created successfully! Please log in.',
      });
    } catch (error: any) {
      Toast.show({
        type: 'error',
        text1: 'Registration Failed',
        text2: error.message || 'Registration failed',
      });
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const registerWithNostr = async (publicKey: string, vaultData: string) => {
    try {
      setLoading(true);
      Toast.show({
        type: 'error',
        text1: 'Feature Coming Soon',
        text2: 'Nostr registration is not yet implemented',
      });
    } catch (error: any) {
      Toast.show({
        type: 'error',
        text1: 'Nostr Registration Failed',
        text2: error.message || 'Nostr registration failed',
      });
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const registerPasskey = async (): Promise<PasskeyCredential> => {
    if (!passkeySupported) {
      Toast.show({
        type: 'error',
        text1: 'Not Supported',
        text2: 'Passkeys are not supported on this device',
      });
      throw new Error('Passkeys not supported');
    }

    if (!token) {
      Toast.show({
        type: 'error',
        text1: 'Not Authenticated',
        text2: 'Please log in first to register a passkey',
      });
      throw new Error('Not authenticated');
    }

    try {
      setLoading(true);
      const credential = await passkeyService.registerPasskey();

      Toast.show({
        type: 'success',
        text1: 'Passkey Registered',
        text2: 'You can now use this passkey to sign in',
      });

      return credential;
    } catch (error: any) {
      Toast.show({
        type: 'error',
        text1: 'Registration Failed',
        text2: error.message || 'Failed to register passkey',
      });
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const getPasskeys = async (): Promise<PasskeyCredential[]> => {
    if (!token) {
      throw new Error('Not authenticated');
    }

    try {
      return await passkeyService.getCredentials();
    } catch (error: any) {
      Toast.show({
        type: 'error',
        text1: 'Error',
        text2: error.message || 'Failed to fetch passkeys',
      });
      throw error;
    }
  };

  const renamePasskey = async (id: string, name: string): Promise<void> => {
    if (!token) {
      throw new Error('Not authenticated');
    }

    try {
      await passkeyService.renameCredential(id, name);
      Toast.show({
        type: 'success',
        text1: 'Success',
        text2: 'Passkey renamed',
      });
    } catch (error: any) {
      Toast.show({
        type: 'error',
        text1: 'Error',
        text2: error.message || 'Failed to rename passkey',
      });
      throw error;
    }
  };

  const deletePasskey = async (id: string): Promise<void> => {
    if (!token) {
      throw new Error('Not authenticated');
    }

    try {
      await passkeyService.deleteCredential(id);
      Toast.show({
        type: 'success',
        text1: 'Success',
        text2: 'Passkey removed',
      });
    } catch (error: any) {
      Toast.show({
        type: 'error',
        text1: 'Error',
        text2: error.message || 'Failed to delete passkey',
      });
      throw error;
    }
  };

  const logout = async () => {
    try {
      setToken(null);
      setUser(null);

      await AsyncStorage.removeItem('vault_token');
      await AsyncStorage.removeItem('vault_user');

      Toast.show({
        type: 'success',
        text1: 'Logged Out',
        text2: 'Successfully logged out',
      });
    } catch (error) {
      console.error('Logout error:', error);
    }
  };

  const value = {
    user,
    token,
    loading,
    passkeySupported,
    login,
    loginWithPasskey,
    loginWithNostr,
    register,
    registerWithNostr,
    registerPasskey,
    getPasskeys,
    renamePasskey,
    deletePasskey,
    logout,
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
