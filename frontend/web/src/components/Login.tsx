import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { useForm } from 'react-hook-form';
import { useAuth } from '../contexts/AuthContext';
import { Shield, Mail, Key, Zap, Fingerprint } from 'lucide-react';
import toast from 'react-hot-toast';

interface LoginForm {
  email: string;
  password: string;
}

interface LightningChallenge {
  k1: string;
  lnurl: string;
  expiresAt: string;
}

export default function Login() {
  const { login, loginWithNostr, getLightningChallenge, loginWithLightning, loginWithWebAuthn, loginWithWebAuthnDiscoverable, isWebAuthnAvailable, loading } = useAuth();
  const [authMethod, setAuthMethod] = useState<'email' | 'nostr' | 'lightning' | 'passkey'>('email');
  const [nostrPublicKey, setNostrPublicKey] = useState('');
  const [lightningAddress, setLightningAddress] = useState('');
  const [lightningChallenge, setLightningChallenge] = useState<LightningChallenge | null>(null);
  const [lightningSignature, setLightningSignature] = useState('');
  const [lightningLinkingKey, setLightningLinkingKey] = useState('');
  const [passkeyEmail, setPasskeyEmail] = useState('');
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

  const onLightningGetChallenge = async () => {
    if (!lightningAddress || !lightningAddress.includes('@')) {
      toast.error('Please enter a valid Lightning address (e.g., user@wallet.com)');
      return;
    }

    try {
      const challenge = await getLightningChallenge(lightningAddress);
      setLightningChallenge(challenge);
      toast.success('Challenge received! Sign with your Lightning wallet');
    } catch (error) {
      // Error handling is done in AuthContext
    }
  };

  const onLightningLogin = async () => {
    if (!lightningChallenge) {
      toast.error('Please get a challenge first');
      return;
    }

    if (!lightningSignature || lightningSignature.length !== 128) {
      toast.error('Please enter a valid 128-character signature');
      return;
    }

    if (!lightningLinkingKey || lightningLinkingKey.length !== 66) {
      toast.error('Please enter a valid 66-character linking key');
      return;
    }

    try {
      await loginWithLightning(lightningChallenge.k1, lightningSignature, lightningLinkingKey);
    } catch (error) {
      // Error handling is done in AuthContext
    }
  };

  const onPasskeyLogin = async () => {
    if (!passkeyEmail || !passkeyEmail.includes('@')) {
      toast.error('Please enter a valid email address');
      return;
    }

    try {
      await loginWithWebAuthn(passkeyEmail);
    } catch (error) {
      // Error handling is done in AuthContext
    }
  };

  const onPasskeyDiscoverableLogin = async () => {
    try {
      await loginWithWebAuthnDiscoverable();
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
            <div className="grid grid-cols-2 gap-1 bg-muted rounded-lg p-1">
              <button
                type="button"
                className={`py-2 px-2 text-sm font-medium rounded-md transition-colors flex items-center justify-center ${
                  authMethod === 'email'
                    ? 'bg-white text-primary shadow-sm dark:bg-gray-800'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
                onClick={() => setAuthMethod('email')}
              >
                <Mail className="w-4 h-4 mr-1" />
                Email
              </button>
              {isWebAuthnAvailable && (
                <button
                  type="button"
                  className={`py-2 px-2 text-sm font-medium rounded-md transition-colors flex items-center justify-center ${
                    authMethod === 'passkey'
                      ? 'bg-white text-primary shadow-sm dark:bg-gray-800'
                      : 'text-muted-foreground hover:text-foreground'
                  }`}
                  onClick={() => setAuthMethod('passkey')}
                >
                  <Fingerprint className="w-4 h-4 mr-1" />
                  Passkey
                </button>
              )}
              <button
                type="button"
                className={`py-2 px-2 text-sm font-medium rounded-md transition-colors flex items-center justify-center ${
                  authMethod === 'nostr'
                    ? 'bg-white text-primary shadow-sm dark:bg-gray-800'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
                onClick={() => setAuthMethod('nostr')}
              >
                <Key className="w-4 h-4 mr-1" />
                Nostr
              </button>
              <button
                type="button"
                className={`py-2 px-2 text-sm font-medium rounded-md transition-colors flex items-center justify-center ${
                  authMethod === 'lightning'
                    ? 'bg-white text-primary shadow-sm dark:bg-gray-800'
                    : 'text-muted-foreground hover:text-foreground'
                }`}
                onClick={() => setAuthMethod('lightning')}
              >
                <Zap className="w-4 h-4 mr-1" />
                Lightning
              </button>
            </div>

            {authMethod === 'email' && (
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
            )}

            {authMethod === 'passkey' && (
              <div className="space-y-4">
                <div className="text-center">
                  <Fingerprint className="w-12 h-12 mx-auto text-primary mb-2" />
                  <p className="text-sm text-muted-foreground">
                    Sign in with your device's built-in authenticator
                  </p>
                </div>

                <button
                  type="button"
                  onClick={onPasskeyDiscoverableLogin}
                  disabled={loading}
                  className="btn-primary w-full"
                >
                  {loading ? 'Authenticating...' : 'Sign in with Passkey'}
                </button>

                <div className="relative">
                  <div className="absolute inset-0 flex items-center">
                    <div className="w-full border-t border-muted"></div>
                  </div>
                  <div className="relative flex justify-center text-xs">
                    <span className="bg-card px-2 text-muted-foreground">or enter email</span>
                  </div>
                </div>

                <div>
                  <label htmlFor="passkeyEmail" className="block text-sm font-medium text-foreground mb-2">
                    Email address
                  </label>
                  <input
                    type="email"
                    value={passkeyEmail}
                    onChange={(e) => setPasskeyEmail(e.target.value)}
                    className="input w-full"
                    placeholder="Enter your email"
                  />
                </div>

                <button
                  type="button"
                  onClick={onPasskeyLogin}
                  disabled={loading || !passkeyEmail}
                  className="btn-secondary w-full"
                >
                  {loading ? 'Authenticating...' : 'Sign in with Email + Passkey'}
                </button>

                <div className="bg-muted/50 p-3 rounded-lg">
                  <p className="text-xs text-muted-foreground">
                    <strong>Note:</strong> Passkeys use Face ID, Touch ID, Windows Hello, or hardware security keys
                    for passwordless authentication.
                  </p>
                </div>
              </div>
            )}

            {authMethod === 'nostr' && (
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

            {authMethod === 'lightning' && (
              <div className="space-y-4">
                {!lightningChallenge ? (
                  <>
                    <div>
                      <label htmlFor="lightningAddress" className="block text-sm font-medium text-foreground mb-2">
                        Lightning Address
                      </label>
                      <input
                        type="text"
                        value={lightningAddress}
                        onChange={(e) => setLightningAddress(e.target.value)}
                        className="input w-full"
                        placeholder="user@wallet.com"
                      />
                      <p className="mt-1 text-xs text-muted-foreground">
                        Enter your Lightning address (e.g., alice@getalby.com)
                      </p>
                    </div>

                    <button
                      type="button"
                      onClick={onLightningGetChallenge}
                      disabled={loading || !lightningAddress}
                      className="btn-primary w-full"
                    >
                      {loading ? 'Getting challenge...' : 'Get Challenge'}
                    </button>
                  </>
                ) : (
                  <>
                    <div className="bg-muted/50 p-3 rounded-lg">
                      <p className="text-xs text-muted-foreground mb-2">
                        <strong>Challenge (k1):</strong>
                      </p>
                      <code className="text-xs break-all">{lightningChallenge.k1}</code>
                    </div>

                    <div>
                      <label htmlFor="lightningSignature" className="block text-sm font-medium text-foreground mb-2">
                        Signature
                      </label>
                      <input
                        type="text"
                        value={lightningSignature}
                        onChange={(e) => setLightningSignature(e.target.value)}
                        className="input w-full font-mono text-sm"
                        placeholder="128-character DER signature"
                        maxLength={128}
                      />
                    </div>

                    <div>
                      <label htmlFor="lightningLinkingKey" className="block text-sm font-medium text-foreground mb-2">
                        Linking Key
                      </label>
                      <input
                        type="text"
                        value={lightningLinkingKey}
                        onChange={(e) => setLightningLinkingKey(e.target.value)}
                        className="input w-full font-mono text-sm"
                        placeholder="66-character compressed public key"
                        maxLength={66}
                      />
                    </div>

                    <div className="flex gap-2">
                      <button
                        type="button"
                        onClick={() => {
                          setLightningChallenge(null);
                          setLightningSignature('');
                          setLightningLinkingKey('');
                        }}
                        className="btn-secondary flex-1"
                      >
                        Back
                      </button>
                      <button
                        type="button"
                        onClick={onLightningLogin}
                        disabled={loading || !lightningSignature || !lightningLinkingKey}
                        className="btn-primary flex-1"
                      >
                        {loading ? 'Signing in...' : 'Sign in'}
                      </button>
                    </div>
                  </>
                )}

                <div className="bg-muted/50 p-3 rounded-lg">
                  <p className="text-xs text-muted-foreground">
                    <strong>Note:</strong> Lightning authentication uses LNURL-auth. Sign the challenge with your
                    Lightning wallet and paste the signature and linking key above.
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