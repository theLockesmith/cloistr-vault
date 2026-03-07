import React, { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { LogOut, Settings, Plus, Search, Key, Zap, Mail, BadgeCheck } from 'lucide-react';

interface LayoutProps {
  children: ReactNode;
}

// Get display name with priority: NIP-05 > Lightning > npub > email
function getUserDisplayName(user: any): string {
  if (user?.nip05_address) return user.nip05_address;
  if (user?.lightning_address) return user.lightning_address;
  if (user?.nostr_pubkey) {
    const pk = user.nostr_pubkey;
    return `npub1${pk.substring(0, 8)}...${pk.substring(56)}`;
  }
  return user?.email || 'User';
}

// Get auth method icon
function getAuthIcon(user: any) {
  if (user?.nip05_address) return <BadgeCheck className="h-4 w-4 text-blue-500" />;
  if (user?.lightning_address) return <Zap className="h-4 w-4 text-yellow-500" />;
  if (user?.nostr_pubkey) return <Key className="h-4 w-4 text-purple-500" />;
  return <Mail className="h-4 w-4 text-muted-foreground" />;
}

export default function Layout({ children }: LayoutProps) {
  const { user, logout } = useAuth();

  return (
    <div className="min-h-screen bg-background">
      {/* Header */}
      <header className="border-b border-border bg-card">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            {/* Logo */}
            <Link to="/" className="flex items-center space-x-2">
              <img src="/cloistr-icon.svg" alt="Cloistr" className="h-8 w-8" />
              <span className="font-bold text-lg">Cloistr Vault</span>
            </Link>

            {/* Search bar */}
            <div className="flex-1 max-w-md mx-8">
              <div className="relative">
                <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <input
                  type="text"
                  placeholder="Search your vault..."
                  className="input w-full pl-10"
                />
              </div>
            </div>

            {/* User menu */}
            <div className="flex items-center space-x-4">
              <button className="btn-primary">
                <Plus className="h-4 w-4 mr-2" />
                Add Item
              </button>
              
              <div className="flex items-center space-x-2 text-sm">
                {getAuthIcon(user)}
                <span className="font-medium">{getUserDisplayName(user)}</span>
              </div>

              <div className="flex items-center space-x-2">
                <Link
                  to="/settings"
                  className="p-2 text-muted-foreground hover:text-foreground rounded-md hover:bg-accent"
                  title="Settings"
                >
                  <Settings className="h-4 w-4" />
                </Link>

                <button
                  onClick={logout}
                  className="p-2 text-muted-foreground hover:text-foreground rounded-md hover:bg-accent"
                  title="Sign out"
                >
                  <LogOut className="h-4 w-4" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </header>

      {/* Main content */}
      <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {children}
      </main>

      {/* Footer */}
      <footer className="border-t border-border bg-card mt-auto">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex items-center justify-between text-sm text-muted-foreground">
            <div className="flex items-center space-x-4">
              <span>© 2024 Cloistr Vault</span>
              <span>•</span>
              <span>Zero-knowledge password manager</span>
            </div>
            
            <div className="flex items-center space-x-4">
              <span className="flex items-center space-x-2">
                <div className="w-2 h-2 bg-green-500 rounded-full"></div>
                <span>Secure connection</span>
              </span>
            </div>
          </div>
        </div>
      </footer>
    </div>
  );
}