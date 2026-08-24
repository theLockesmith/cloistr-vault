import React from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider as NostrAuthProvider, SignerRecovery } from '@cloistr/ui';
import { AuthProvider, useAuth } from './contexts/AuthContext';
import { CryptoProvider } from './contexts/CryptoContext';
import { VaultProvider } from './contexts/VaultContext';
import Login from './components/Login';
import Register from './components/Register';
import Dashboard from './components/Dashboard';
import Settings from './components/Settings';
import Layout from './components/Layout';
import UnlockModal from './components/UnlockModal';

/**
 * Protected Route component.
 *
 * THREE states, not two:
 *   loading             — spinner while vault token / signer probe resolves
 *   signerProbeError    — probe failed transiently; show SignerRecovery, NOT login
 *   !user               — genuinely no session; redirect to /login
 *
 * The third path is the bug fixed on this branch: a network error during the
 * startup signer probe set signerProbeError but the old code only saw !user and
 * redirected to /login. The cookie was still valid — the user was NOT signed
 * out. Sending them to a credential prompt for a still-valid session is what
 * read as "the app randomly logs me out".
 *
 * Part 4 (visibilitychange reconnect) is implemented in AuthContext.tsx as a
 * visibilitychange listener that re-runs doSignerProbe() when the page regains
 * focus and signerProbeError is set. @cloistr/ui's useRelayReconnect hook
 * (which handles NIP-46 relay WebSocket reconnect) will wire here once 0.27.0
 * is published; at 0.26.0 the hook is not yet exported.
 */
function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { user, loading, signerProbeError, retrySignerProbe } = useAuth();
  const [retrying, setRetrying] = React.useState(false);

  if (loading) {
    return (
      <div className="min-h-dvh flex items-center justify-center bg-cloistr-bg">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary"></div>
      </div>
    );
  }

  // Transient signer-probe failure: the session may still be valid. Show the
  // recovery screen so the user can retry or navigate back. NEVER redirect to
  // /login here — that would destroy a session that was never actually invalid.
  if (signerProbeError !== null) {
    const handleRetry = async () => {
      setRetrying(true);
      try {
        await retrySignerProbe();
      } finally {
        setRetrying(false);
      }
    };

    return (
      <div className="min-h-dvh flex items-center justify-center bg-cloistr-bg p-4">
        <div className="w-full max-w-md">
          <SignerRecovery
            error={signerProbeError}
            retrying={retrying}
            onRetry={handleRetry}
          />
        </div>
      </div>
    );
  }

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  return (
    <>
      <UnlockModal />
      <Layout>{children}</Layout>
    </>
  );
}

// Public Route component - redirects to dashboard if already authenticated
function PublicRoute({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();

  if (loading) {
    return (
      <div className="min-h-dvh flex items-center justify-center bg-cloistr-bg">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary"></div>
      </div>
    );
  }

  if (user) {
    return <Navigate to="/" replace />;
  }

  return <>{children}</>;
}

// App Routes component - must be inside AuthProvider
function AppRoutes() {
  return (
    <Routes>
      {/* Public routes */}
      <Route
        path="/login"
        element={
          <PublicRoute>
            <Login />
          </PublicRoute>
        }
      />
      <Route
        path="/register"
        element={
          <PublicRoute>
            <Register />
          </PublicRoute>
        }
      />

      {/* Protected routes */}
      <Route
        path="/"
        element={
          <ProtectedRoute>
            <Dashboard />
          </ProtectedRoute>
        }
      />
      <Route
        path="/settings"
        element={
          <ProtectedRoute>
            <Settings />
          </ProtectedRoute>
        }
      />

      {/* Catch all - redirect to home */}
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

function App() {
  return (
    // NostrAuthProvider satisfies the useNostrAuth() call inside @cloistr/ui Header.
    // Vault manages its own session via AuthContext (vault token + signer cookie);
    // the Nostr auth context is idle — it never takes over vault's auth flow.
    <NostrAuthProvider>
      <AuthProvider>
        <CryptoProvider>
          <VaultProvider>
            <AppRoutes />
          </VaultProvider>
        </CryptoProvider>
      </AuthProvider>
    </NostrAuthProvider>
  );
}

export default App;
