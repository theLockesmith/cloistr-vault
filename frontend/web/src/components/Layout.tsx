import React, { ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { Header, Footer } from '@cloistr/ui';
import { useAuth } from '../contexts/AuthContext';
import { Plus, Search } from 'lucide-react';

interface LayoutProps {
  children: ReactNode;
}

export default function Layout({ children }: LayoutProps) {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  const isAuthenticated = user !== null;
  // Prefer nostr_pubkey / pubkey field for the Header user-menu display.
  const pubkey = user?.nostr_pubkey ?? user?.pubkey ?? undefined;

  return (
    <div className="min-h-screen bg-background flex flex-col">
      {/* Shared Cloistr Header: app-switcher, theme toggle, wordmark, user menu */}
      <Header
        activeServiceId="vault"
        settingsUrl="/settings"
        auth={{
          authenticated: isAuthenticated,
          pubkey,
          // onSignIn navigates to vault's own login page — no shared modal needed.
          onSignIn: () => navigate('/login'),
          onLogout: logout,
        }}
      >
        {/* Vault-specific controls inlined as header children (right-side slot) */}
        {isAuthenticated && (
          <>
            <div className="relative hidden sm:block">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <input
                type="text"
                placeholder="Search your vault..."
                className="input pl-10 w-56"
              />
            </div>
            <button className="btn-primary flex items-center">
              <Plus className="h-4 w-4 mr-1" />
              Add Item
            </button>
          </>
        )}
      </Header>

      {/* Main content */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {children}
      </main>

      {/* Shared Cloistr Footer */}
      <Footer
        copyrightHolder="Cloistr"
        copyrightYear={2024}
        links={[
          { label: 'Privacy', href: 'https://cloistr.xyz/privacy', external: true },
          { label: 'Terms', href: 'https://cloistr.xyz/terms', external: true },
        ]}
      />
    </div>
  );
}
