import React, { useState, useEffect } from 'react';
import { Lock, Eye, EyeOff, Shield, LogOut } from 'lucide-react';
import { useVault } from '../contexts/VaultContext';
import { useAuth } from '../contexts/AuthContext';

interface UnlockModalProps {
  onUnlock?: () => void;
}

export default function UnlockModal({ onUnlock }: UnlockModalProps) {
  const { isLocked, unlock, loading } = useVault();
  const { logout, user } = useAuth();
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    // Reset form when modal opens
    if (isLocked) {
      setPassword('');
      setError('');
      setShowPassword(false);
    }
  }, [isLocked]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!password) {
      setError('Please enter your master password');
      return;
    }

    const success = await unlock(password);
    if (success) {
      setPassword('');
      onUnlock?.();
    } else {
      setError('Invalid master password');
      setPassword('');
    }
  };

  const handleLogout = () => {
    logout();
  };

  if (!isLocked) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-background">
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-16 h-16 bg-primary/10 rounded-full mb-4">
            <Shield className="h-8 w-8 text-primary" />
          </div>
          <h1 className="text-2xl font-bold text-foreground">Cloistr Vault</h1>
          <p className="text-muted-foreground mt-2">
            {user?.email || user?.display_name || 'Your vault is locked'}
          </p>
        </div>

        <div className="card">
          <div className="card-header">
            <div className="flex items-center space-x-2">
              <Lock className="h-5 w-5 text-primary" />
              <h2 className="card-title">Unlock Your Vault</h2>
            </div>
            <p className="card-description">
              Enter your master password to access your passwords and secure notes.
            </p>
          </div>

          <form onSubmit={handleSubmit} className="card-content space-y-4">
            <div className="space-y-2">
              <label htmlFor="masterPassword" className="text-sm font-medium">
                Master Password
              </label>
              <div className="relative">
                <input
                  id="masterPassword"
                  type={showPassword ? 'text' : 'password'}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Enter your master password"
                  className={`input w-full pr-10 ${error ? 'border-red-500' : ''}`}
                  autoFocus
                  disabled={loading}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute inset-y-0 right-0 flex items-center pr-3 text-muted-foreground hover:text-foreground"
                  tabIndex={-1}
                >
                  {showPassword ? (
                    <EyeOff className="h-4 w-4" />
                  ) : (
                    <Eye className="h-4 w-4" />
                  )}
                </button>
              </div>
              {error && <p className="text-sm text-red-500">{error}</p>}
            </div>

            <button
              type="submit"
              className="btn-primary w-full"
              disabled={loading || !password}
            >
              {loading ? (
                <>
                  <div className="animate-spin rounded-full h-4 w-4 border-2 border-white border-t-transparent mr-2" />
                  Unlocking...
                </>
              ) : (
                <>
                  <Lock className="h-4 w-4 mr-2" />
                  Unlock Vault
                </>
              )}
            </button>
          </form>

          <div className="card-footer">
            <button
              type="button"
              onClick={handleLogout}
              className="btn-ghost w-full text-muted-foreground"
            >
              <LogOut className="h-4 w-4 mr-2" />
              Sign out and use a different account
            </button>
          </div>
        </div>

        <p className="text-center text-xs text-muted-foreground mt-4">
          Your vault is protected with zero-knowledge encryption.
          <br />
          We never see your master password or unencrypted data.
        </p>
      </div>
    </div>
  );
}
