import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { useAuth } from '../contexts/AuthContext';
import { Shield, Mail, Lock, Key } from 'lucide-react';
import toast from 'react-hot-toast';

interface LoginForm {
  email: string;
  password: string;
}

export default function Login() {
  const { login, loginWithNostr, loading } = useAuth();
  const [authMethod, setAuthMethod] = useState<'email' | 'nostr'>('email');
  const [nostrPublicKey, setNostrPublicKey] = useState('');
  const { register, handleSubmit, formState: { errors } } = useForm<LoginForm>();

  const onEmailSubmit = async (data: LoginForm) => {
    try {
      await login(data.email, data.password);
    } catch (error) {
      // Error handling is done in AuthContext
    }
  };

  const onNostrLogin = async () => {
    if (!nostrPublicKey || nostrPublicKey.length !== 64) {
      toast.error('Please enter a valid 64-character Nostr public key');
      return;
    }

    try {
      // For demo purposes, we'll simulate the challenge/signature flow
      // In a real implementation, this would involve:
      // 1. Get challenge from server
      // 2. Sign with Nostr extension/client
      // 3. Submit signature
      toast.error('Nostr login requires a Nostr client/extension to sign challenges');
    } catch (error) {
      // Error handling is done in AuthContext
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-blue-50 to-indigo-100 dark:from-gray-900 dark:to-gray-800">
      <div className="max-w-md w-full space-y-8 p-8">
        <div className="card">
          <div className="card-header text-center">
            <div className="mx-auto h-12 w-12 bg-primary/10 rounded-full flex items-center justify-center mb-4">
              <Shield className="h-6 w-6 text-primary" />
            </div>
            <h2 className="card-title">Welcome to Coldforge Vault</h2>
            <p className="card-description">
              Zero-knowledge password manager
            </p>
          </div>

          <div className="card-content space-y-6">
            {/* Auth Method Toggle */}
            <div className="flex bg-muted rounded-lg p-1">
              <button
                type="button"
                className={`flex-1 py-2 px-4 text-sm font-medium rounded-md transition-colors ${
                  authMethod === 'email'
                    ? 'bg-white text-primary shadow-sm dark:bg-gray-800'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
                onClick={() => setAuthMethod('email')}
              >
                <Mail className="w-4 h-4 inline mr-2" />
                Email
              </button>
              <button
                type="button"
                className={`flex-1 py-2 px-4 text-sm font-medium rounded-md transition-colors ${
                  authMethod === 'nostr'
                    ? 'bg-white text-primary shadow-sm dark:bg-gray-800'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
                onClick={() => setAuthMethod('nostr')}
              >
                <Key className="w-4 h-4 inline mr-2" />
                Nostr
              </button>
            </div>

            {authMethod === 'email' ? (
              <form onSubmit={handleSubmit(onEmailSubmit)} className="space-y-4">
                <div>
                  <label htmlFor="email" className="block text-sm font-medium text-foreground mb-2">
                    Email address
                  </label>
                  <input
                    {...register('email', {
                      required: 'Email is required',
                      pattern: {
                        value: /\S+@\S+\.\S+/,
                        message: 'Please enter a valid email',
                      },
                    })}
                    type="email"
                    className="input w-full"
                    placeholder="Enter your email"
                  />
                  {errors.email && (
                    <p className="mt-1 text-sm text-destructive">{errors.email.message}</p>
                  )}
                </div>

                <div>
                  <label htmlFor="password" className="block text-sm font-medium text-foreground mb-2">
                    Password
                  </label>
                  <input
                    {...register('password', {
                      required: 'Password is required',
                      minLength: {
                        value: 8,
                        message: 'Password must be at least 8 characters',
                      },
                    })}
                    type="password"
                    className="input w-full"
                    placeholder="Enter your password"
                  />
                  {errors.password && (
                    <p className="mt-1 text-sm text-destructive">{errors.password.message}</p>
                  )}
                </div>

                <button
                  type="submit"
                  disabled={loading}
                  className="btn-primary w-full"
                >
                  {loading ? 'Signing in...' : 'Sign in'}
                </button>
              </form>
            ) : (
              <div className="space-y-4">
                <div>
                  <label htmlFor="nostrKey" className="block text-sm font-medium text-foreground mb-2">
                    Nostr Public Key
                  </label>
                  <input
                    type="text"
                    value={nostrPublicKey}
                    onChange={(e) => setNostrPublicKey(e.target.value)}
                    className="input w-full font-mono text-sm"
                    placeholder="Enter your 64-character Nostr public key"
                    maxLength={64}
                  />
                  <p className="mt-1 text-xs text-muted-foreground">
                    Your public key should be 64 hexadecimal characters
                  </p>
                </div>

                <button
                  type="button"
                  onClick={onNostrLogin}
                  disabled={loading || !nostrPublicKey}
                  className="btn-primary w-full"
                >
                  {loading ? 'Signing in...' : 'Sign in with Nostr'}
                </button>

                <div className="bg-muted/50 p-3 rounded-lg">
                  <p className="text-xs text-muted-foreground">
                    <strong>Note:</strong> Nostr authentication requires a compatible Nostr client or browser extension 
                    to sign the authentication challenge.
                  </p>
                </div>
              </div>
            )}

            <div className="text-center">
              <p className="text-sm text-muted-foreground">
                Don't have an account?{' '}
                <Link to="/register" className="text-primary hover:underline">
                  Create one here
                </Link>
              </p>
            </div>
          </div>
        </div>

        <div className="text-center">
          <p className="text-xs text-muted-foreground">
            Your data is encrypted locally and never stored unencrypted on our servers
          </p>
        </div>
      </div>
    </div>
  );
}