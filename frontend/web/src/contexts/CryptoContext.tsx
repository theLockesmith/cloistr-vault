import React, { createContext, useContext, ReactNode } from 'react';
import { generatePassword as generateSecurePassword } from '../crypto/random';

/**
 * Password generation for the UI.
 *
 * Vault encryption used to live here too, in the v1 format: PBKDF2 into
 * CryptoJS.AES with a string key, which is unauthenticated AES-256-CBC behind
 * a single round of MD5. Those functions have been removed rather than kept
 * around, so nothing in the app can write a v1 vault again. Encryption now
 * lives in src/crypto, and the only remaining v1 code is the decrypt-only
 * reader used to migrate existing vaults (src/crypto/legacy.ts).
 */
interface CryptoContextType {
  generatePassword: (length?: number, includeSpecial?: boolean) => string;
}

const CryptoContext = createContext<CryptoContextType | undefined>(undefined);

export function CryptoProvider({ children }: { children: ReactNode }) {
  const value: CryptoContextType = {
    generatePassword: generateSecurePassword,
  };

  return <CryptoContext.Provider value={value}>{children}</CryptoContext.Provider>;
}

export function useCrypto() {
  const context = useContext(CryptoContext);
  if (context === undefined) {
    throw new Error('useCrypto must be used within a CryptoProvider');
  }
  return context;
}
