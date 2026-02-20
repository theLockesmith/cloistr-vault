import React, { useState } from 'react';
import { useAuth } from '../contexts/AuthContext';
import { Shield, Key, Zap, CheckCircle, AlertCircle, Search, ArrowLeft } from 'lucide-react';
import { Link } from 'react-router-dom';
import toast from 'react-hot-toast';

export default function Settings() {
  const { user, verifyNIP05, lookupNIP05, loading } = useAuth();
  const [nip05Input, setNip05Input] = useState('');
  const [lookupResult, setLookupResult] = useState<{
    nip05_address: string;
    pubkey: string;
    relays: string[];
  } | null>(null);
  const [lookupError, setLookupError] = useState<string | null>(null);

  const isNostrUser = user?.auth_method === 'nostr' || user?.nostr_pubkey;

  const handleLookup = async () => {
    if (!nip05Input || !nip05Input.includes('@')) {
      toast.error('Please enter a valid NIP-05 address (e.g., alice@domain.com)');
      return;
    }

    setLookupError(null);
    setLookupResult(null);

    try {
      const result = await lookupNIP05(nip05Input);
      setLookupResult(result);

      // Check if pubkey matches user's pubkey
      if (user?.nostr_pubkey && result.pubkey !== user.nostr_pubkey) {
        setLookupError('This NIP-05 address resolves to a different pubkey than yours');
      }
    } catch (error: any) {
      setLookupError(error.response?.data?.error || 'Lookup failed');
    }
  };

  const handleVerify = async () => {
    if (!nip05Input || !nip05Input.includes('@')) {
      toast.error('Please enter a valid NIP-05 address');
      return;
    }

    try {
      await verifyNIP05(nip05Input);
      setNip05Input('');
      setLookupResult(null);
    } catch (error) {
      // Error handled in context
    }
  };

  const formatPubkey = (pubkey: string): string => {
    if (!pubkey || pubkey.length < 16) return pubkey;
    return `${pubkey.slice(0, 8)}...${pubkey.slice(-8)}`;
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center space-x-4">
        <Link to="/" className="p-2 hover:bg-accent rounded-md transition-colors">
          <ArrowLeft className="h-5 w-5" />
        </Link>
        <div>
          <h1 className="text-2xl font-bold text-foreground">Settings</h1>
          <p className="text-muted-foreground">Manage your account and identity</p>
        </div>
      </div>

      {/* Identity Section */}
      <div className="card">
        <div className="card-header">
          <div className="flex items-center space-x-2">
            <Shield className="h-5 w-5 text-primary" />
            <h2 className="card-title">Identity</h2>
          </div>
          <p className="card-description">Your authentication methods and verified identities</p>
        </div>

        <div className="card-content space-y-4">
          {/* Current Auth Method */}
          <div className="p-4 bg-muted/50 rounded-lg">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-medium">Authentication Method</p>
                <p className="text-sm text-muted-foreground capitalize">
                  {user?.auth_method || 'email'}
                </p>
              </div>
              <div className="flex items-center space-x-2">
                {user?.auth_method === 'nostr' && <Key className="h-5 w-5 text-purple-500" />}
                {user?.auth_method === 'lightning_address' && <Zap className="h-5 w-5 text-yellow-500" />}
                {(!user?.auth_method || user?.auth_method === 'email') && <Shield className="h-5 w-5 text-blue-500" />}
              </div>
            </div>
          </div>

          {/* Nostr Pubkey */}
          {user?.nostr_pubkey && (
            <div className="p-4 bg-muted/50 rounded-lg">
              <p className="text-sm font-medium mb-1">Nostr Public Key</p>
              <code className="text-xs text-muted-foreground break-all">
                {user.nostr_pubkey}
              </code>
            </div>
          )}

          {/* Lightning Address */}
          {user?.lightning_address && (
            <div className="p-4 bg-muted/50 rounded-lg">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium">Lightning Address</p>
                  <p className="text-sm text-muted-foreground">{user.lightning_address}</p>
                </div>
                <Zap className="h-5 w-5 text-yellow-500" />
              </div>
            </div>
          )}

          {/* Verified NIP-05 */}
          {user?.nip05_address && (
            <div className="p-4 bg-green-500/10 border border-green-500/20 rounded-lg">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium text-green-700 dark:text-green-400">Verified NIP-05</p>
                  <p className="text-sm text-green-600 dark:text-green-500">{user.nip05_address}</p>
                </div>
                <CheckCircle className="h-5 w-5 text-green-500" />
              </div>
            </div>
          )}
        </div>
      </div>

      {/* NIP-05 Verification Section */}
      {isNostrUser && (
        <div className="card">
          <div className="card-header">
            <div className="flex items-center space-x-2">
              <Key className="h-5 w-5 text-purple-500" />
              <h2 className="card-title">NIP-05 Verification</h2>
            </div>
            <p className="card-description">
              Link a human-readable identifier to your Nostr pubkey (e.g., alice@domain.com)
            </p>
          </div>

          <div className="card-content space-y-4">
            <div className="bg-muted/50 p-4 rounded-lg">
              <h4 className="text-sm font-medium mb-2">What is NIP-05?</h4>
              <p className="text-sm text-muted-foreground">
                NIP-05 allows you to verify that you own an internet identifier (like alice@domain.com).
                The domain owner publishes your Nostr pubkey at a well-known URL, proving the link between
                your human-readable address and your cryptographic identity.
              </p>
            </div>

            {/* NIP-05 Input */}
            <div className="space-y-2">
              <label className="text-sm font-medium">NIP-05 Address</label>
              <div className="flex space-x-2">
                <input
                  type="text"
                  value={nip05Input}
                  onChange={(e) => setNip05Input(e.target.value)}
                  placeholder="alice@domain.com"
                  className="input flex-1"
                />
                <button
                  onClick={handleLookup}
                  disabled={loading || !nip05Input}
                  className="btn-secondary"
                  title="Look up this NIP-05 address"
                >
                  <Search className="h-4 w-4" />
                </button>
              </div>
              <p className="text-xs text-muted-foreground">
                Enter a NIP-05 address to look up, then verify if it matches your pubkey
              </p>
            </div>

            {/* Lookup Result */}
            {lookupResult && (
              <div className="p-4 bg-muted/50 rounded-lg space-y-3">
                <div className="flex items-center space-x-2">
                  <CheckCircle className="h-4 w-4 text-green-500" />
                  <span className="text-sm font-medium">NIP-05 Found</span>
                </div>

                <div className="space-y-2 text-sm">
                  <div>
                    <span className="text-muted-foreground">Address: </span>
                    <span className="font-mono">{lookupResult.nip05_address}</span>
                  </div>
                  <div>
                    <span className="text-muted-foreground">Pubkey: </span>
                    <code className="text-xs">{formatPubkey(lookupResult.pubkey)}</code>
                  </div>
                  {lookupResult.relays && lookupResult.relays.length > 0 && (
                    <div>
                      <span className="text-muted-foreground">Relays: </span>
                      <span className="text-xs">{lookupResult.relays.length} configured</span>
                    </div>
                  )}
                </div>

                {/* Pubkey match check */}
                {user?.nostr_pubkey && (
                  <div className={`p-2 rounded ${
                    lookupResult.pubkey === user.nostr_pubkey
                      ? 'bg-green-500/10 text-green-700 dark:text-green-400'
                      : 'bg-red-500/10 text-red-700 dark:text-red-400'
                  }`}>
                    <div className="flex items-center space-x-2 text-sm">
                      {lookupResult.pubkey === user.nostr_pubkey ? (
                        <>
                          <CheckCircle className="h-4 w-4" />
                          <span>Pubkey matches your account</span>
                        </>
                      ) : (
                        <>
                          <AlertCircle className="h-4 w-4" />
                          <span>Pubkey does not match your account</span>
                        </>
                      )}
                    </div>
                  </div>
                )}
              </div>
            )}

            {/* Lookup Error */}
            {lookupError && (
              <div className="p-4 bg-red-500/10 border border-red-500/20 rounded-lg">
                <div className="flex items-center space-x-2 text-red-700 dark:text-red-400">
                  <AlertCircle className="h-4 w-4" />
                  <span className="text-sm">{lookupError}</span>
                </div>
              </div>
            )}

            {/* Verify Button */}
            <button
              onClick={handleVerify}
              disabled={loading || !nip05Input || !lookupResult || lookupResult.pubkey !== user?.nostr_pubkey}
              className="btn-primary w-full"
            >
              {loading ? 'Verifying...' : 'Verify & Link NIP-05'}
            </button>

            {!user?.nip05_address && (
              <p className="text-xs text-muted-foreground text-center">
                Once verified, your NIP-05 address will be displayed as your identity
              </p>
            )}
          </div>
        </div>
      )}

      {/* Non-Nostr User Message */}
      {!isNostrUser && (
        <div className="card">
          <div className="card-header">
            <div className="flex items-center space-x-2">
              <Key className="h-5 w-5 text-muted-foreground" />
              <h2 className="card-title text-muted-foreground">NIP-05 Verification</h2>
            </div>
          </div>
          <div className="card-content">
            <div className="p-4 bg-muted/50 rounded-lg text-center">
              <p className="text-sm text-muted-foreground">
                NIP-05 verification is available for Nostr users.
                Sign in with your Nostr key to verify a NIP-05 address.
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
