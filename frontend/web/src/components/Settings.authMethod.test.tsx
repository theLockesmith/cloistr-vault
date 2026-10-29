/**
 * Settings names how the user signed in. A Nostr user whose stored profile
 * carries a pubkey but no auth_method was shown "email" (found by the WV4
 * live sweep for a signer-session owner).
 */
import React from 'react';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Settings from './Settings';

let mockUser: any = null;

vi.mock('../contexts/AuthContext', () => ({
  useAuth: () => ({
    user: mockUser,
    verifyNIP05: vi.fn(),
    lookupNIP05: vi.fn(),
    registerWebAuthnCredential: vi.fn(),
    listWebAuthnCredentials: vi.fn().mockResolvedValue([]),
    deleteWebAuthnCredential: vi.fn(),
    updateWebAuthnCredential: vi.fn(),
    isWebAuthnAvailable: false,
    loading: false,
  }),
}));

vi.mock('../contexts/VaultContext', () => ({
  useVault: () => ({
    isLocked: true,
    enrolledPasskeys: [],
    enrollPasskeyUnlock: vi.fn(),
    revokePasskeyUnlock: vi.fn(),
  }),
}));

function methodShown() {
  const label = screen.getByText('Authentication Method');
  return label.nextElementSibling?.textContent;
}

function renderSettings(user: any) {
  mockUser = user;
  render(
    <MemoryRouter>
      <Settings />
    </MemoryRouter>
  );
}

describe('Settings authentication method', () => {
  it('shows nostr for a user with a pubkey and no auth_method', () => {
    renderSettings({ pubkey: 'a'.repeat(64), nostr_pubkey: 'a'.repeat(64) });
    expect(methodShown()).toBe('nostr');
  });

  it('shows the auth_method the server reported', () => {
    renderSettings({ email: 'a@b.c', auth_method: 'webauthn' });
    expect(methodShown()).toBe('webauthn');
  });

  it('shows email for an email user', () => {
    renderSettings({ email: 'a@b.c', auth_method: 'email' });
    expect(methodShown()).toBe('email');
  });
});
