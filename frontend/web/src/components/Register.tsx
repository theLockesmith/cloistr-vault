import React, { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { useAuth } from '../contexts/AuthContext';
import { useCrypto } from '../contexts/CryptoContext';
import { Mail, Lock, Key, Eye, EyeOff } from 'lucide-react';
import toast from 'react-hot-toast';

interface RegisterForm {
  email: string;
  password: string;
  confirmPassword: string;
}

export default function Register() {
  const { register: registerUser, loading } = useAuth();
  const { encryptVault } = useCrypto();
  const navigate = useNavigate();
  const [authMethod, setAuthMethod] = useState<'email' | 'nostr'>('email');
  const [nostrPublicKey, setNostrPublicKey] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);
  
  const { register, handleSubmit, watch, formState: { errors } } = useForm<RegisterForm>();
  const watchPassword = watch('password');

  const createInitialVault = () => {
    return {
      entries: [
        {
          id: '1',
          type: 'login' as const,
          name: 'Welcome to Cloistr Vault',
          fields: {
            username: 'demo@example.com',
            password: 'This is encrypted locally!',
            url: 'https://cloistr.com'
          },
          notes: 'This is your first vault entry. All data is encrypted on your device before being sent to our servers.',
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
          favorite: true
        }
      ],
      folders: []
    };
  };

  const onEmailSubmit = async (data: RegisterForm) => {
    try {
      // Create initial vault data
      const initialVault = createInitialVault();
      
      // Encrypt the vault with the user's password
      const encryptedVaultData = encryptVault(initialVault, data.password);
      
      await registerUser(data.email, data.password, encryptedVaultData);
      navigate('/login');
    } catch (error) {
      // Error handling is done in AuthContext
    }
  };

  const onNostrRegister = async () => {
    if (!nostrPublicKey || nostrPublicKey.length !== 64) {
      toast.error('Please enter a valid 64-character Nostr public key');
      return;
    }

    try {
      // For Nostr registration, we need to create a vault encrypted with the public key
      // In a real implementation, this would be more sophisticated
      const initialVault = createInitialVault();
      const encryptedVaultData = encryptVault(initialVault, nostrPublicKey);
      
      toast.error('Nostr registration requires additional setup - feature coming soon');
      
      // await registerWithNostr(nostrPublicKey, encryptedVaultData);
      // navigate('/login');
    } catch (error) {
      // Error handling is done in AuthContext
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-blue-50 to-indigo-100 dark:from-gray-900 dark:to-gray-800">
      <div className="max-w-md w-full space-y-8 p-8">
        <div className="card">
          <div className="card-header text-center">
            <div className="mx-auto h-12 w-12 mb-4">
              <img src="/cloistr-icon.svg" alt="Cloistr" className="h-12 w-12" />
            </div>
            <h2 className="card-title">Create Your Vault</h2>
            <p className="card-description">
              Start securing your passwords with zero-knowledge encryption
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
                    Master Password
                  </label>
                  <div className="relative">
                    <input
                      {...register('password', {
                        required: 'Password is required',
                        minLength: {
                          value: 8,
                          message: 'Password must be at least 8 characters',
                        },
                        pattern: {
                          value: /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)/,
                          message: 'Password must contain uppercase, lowercase, and number',
                        },
                      })}
                      type={showPassword ? 'text' : 'password'}
                      className="input w-full pr-10"
                      placeholder="Create a strong master password"
                    />
                    <button
                      type="button"
                      className="absolute inset-y-0 right-0 pr-3 flex items-center"
                      onClick={() => setShowPassword(!showPassword)}
                    >
                      {showPassword ? (
                        <EyeOff className="h-4 w-4 text-muted-foreground" />
                      ) : (
                        <Eye className="h-4 w-4 text-muted-foreground" />
                      )}
                    </button>
                  </div>
                  {errors.password && (
                    <p className="mt-1 text-sm text-destructive">{errors.password.message}</p>
                  )}
                </div>

                <div>
                  <label htmlFor="confirmPassword" className="block text-sm font-medium text-foreground mb-2">
                    Confirm Password
                  </label>
                  <div className="relative">
                    <input
                      {...register('confirmPassword', {
                        required: 'Please confirm your password',
                        validate: (value) => value === watchPassword || 'Passwords do not match',
                      })}
                      type={showConfirmPassword ? 'text' : 'password'}
                      className="input w-full pr-10"
                      placeholder="Confirm your master password"
                    />
                    <button
                      type="button"
                      className="absolute inset-y-0 right-0 pr-3 flex items-center"
                      onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                    >
                      {showConfirmPassword ? (
                        <EyeOff className="h-4 w-4 text-muted-foreground" />
                      ) : (
                        <Eye className="h-4 w-4 text-muted-foreground" />
                      )}
                    </button>
                  </div>
                  {errors.confirmPassword && (
                    <p className="mt-1 text-sm text-destructive">{errors.confirmPassword.message}</p>
                  )}
                </div>

                <div className="bg-muted/50 p-3 rounded-lg">
                  <h4 className="text-sm font-medium text-foreground mb-2">🔒 Zero-Knowledge Security</h4>
                  <ul className="text-xs text-muted-foreground space-y-1">
                    <li>• Your master password encrypts all vault data locally</li>
                    <li>• We never store or have access to your unencrypted data</li>
                    <li>• If you lose your password, your data cannot be recovered</li>
                  </ul>
                </div>

                <button
                  type="submit"
                  disabled={loading}
                  className="btn-primary w-full"
                >
                  {loading ? 'Creating account...' : 'Create Account'}
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

                <div className="bg-muted/50 p-3 rounded-lg">
                  <h4 className="text-sm font-medium text-foreground mb-2">🔑 Nostr Integration</h4>
                  <ul className="text-xs text-muted-foreground space-y-1">
                    <li>• Your vault will be encrypted using your Nostr identity</li>
                    <li>• Requires a Nostr client for authentication</li>
                    <li>• Currently in development - coming soon</li>
                  </ul>
                </div>

                <button
                  type="button"
                  onClick={onNostrRegister}
                  disabled={loading || !nostrPublicKey}
                  className="btn-primary w-full"
                >
                  {loading ? 'Creating account...' : 'Create Account with Nostr'}
                </button>
              </div>
            )}

            <div className="text-center">
              <p className="text-sm text-muted-foreground">
                Already have an account?{' '}
                <Link to="/login" className="text-primary hover:underline">
                  Sign in here
                </Link>
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}