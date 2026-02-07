import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';
import AsyncStorage from '@react-native-async-storage/async-storage';
import Toast from 'react-native-toast-message';

// Mock API service - in real app would use axios
class ApiService {
  static async post(endpoint: string, data: any) {
    // Simulate API call
    await new Promise(resolve => setTimeout(resolve, 1000));
    
    if (endpoint === '/auth/login' && data.email === 'demo@example.com' && data.password === 'demo123') {
      return {
        data: {
          token: 'demo_token_123',
          user: {
            id: '1',
            email: 'demo@example.com',
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
          expires_at: new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString(),
        }
      };
    }
    
    throw new Error('Invalid credentials');
  }
}

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

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

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

  const login = async (email: string, password: string) => {
    try {
      setLoading(true);
      const response = await ApiService.post('/auth/login', {
        method: 'email',
        email,
        password,
      });

      const { token: newToken, user: newUser } = response.data;
      
      setToken(newToken);
      setUser(newUser);
      
      await AsyncStorage.setItem('vault_token', newToken);
      await AsyncStorage.setItem('vault_user', JSON.stringify(newUser));
      
      Toast.show({
        type: 'success',
        text1: 'Success',
        text2: 'Successfully logged in!',
      });
    } catch (error: any) {
      Toast.show({
        type: 'error',
        text1: 'Login Failed',
        text2: error.message || 'Login failed',
      });
      throw error;
    } finally {
      setLoading(false);
    }
  };

  const loginWithNostr = async (publicKey: string, signature: string, challenge: string) => {
    try {
      setLoading(true);
      // Simulate Nostr login
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
      // Simulate registration
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
      // Simulate Nostr registration
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