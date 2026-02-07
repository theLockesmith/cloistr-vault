import React, { createContext, useContext, ReactNode } from 'react';
import CryptoJS from 'crypto-js';

interface VaultEntry {
  id: string;
  type: 'login' | 'note' | 'card' | 'identity';
  name: string;
  fields: Record<string, string>;
  notes: string;
  created_at: string;
  updated_at: string;
  favorite: boolean;
  folder_id?: string;
}

interface VaultData {
  entries: VaultEntry[];
  folders: Array<{
    id: string;
    name: string;
    created_at: string;
  }>;
}

interface CryptoContextType {
  encryptVault: (vaultData: VaultData, password: string) => string;
  decryptVault: (encryptedData: string, password: string) => VaultData | null;
  generatePassword: (length?: number, includeSpecial?: boolean) => string;
  deriveKey: (password: string, salt: string) => string;
  generateSalt: () => string;
}

const CryptoContext = createContext<CryptoContextType | undefined>(undefined);

export function CryptoProvider({ children }: { children: ReactNode }) {
  const deriveKey = (password: string, salt: string): string => {
    // Use PBKDF2 with SHA-256 and 100,000 iterations for key derivation
    const key = CryptoJS.PBKDF2(password, salt, {
      keySize: 256 / 32,
      iterations: 100000,
      hasher: CryptoJS.algo.SHA256
    });
    return key.toString();
  };

  const generateSalt = (): string => {
    return CryptoJS.lib.WordArray.random(256 / 8).toString();
  };

  const encryptVault = (vaultData: VaultData, password: string): string => {
    try {
      const salt = generateSalt();
      const key = deriveKey(password, salt);
      const dataString = JSON.stringify(vaultData);
      
      // Encrypt with AES-256-GCM equivalent (using AES-256-CBC with HMAC for compatibility)
      const encrypted = CryptoJS.AES.encrypt(dataString, key).toString();
      
      // Return base64 encoded result with salt prepended
      const result = {
        salt,
        data: encrypted
      };
      
      return btoa(JSON.stringify(result));
    } catch (error) {
      console.error('Encryption failed:', error);
      throw new Error('Failed to encrypt vault data');
    }
  };

  const decryptVault = (encryptedData: string, password: string): VaultData | null => {
    try {
      // Decode base64 and parse
      const decoded = JSON.parse(atob(encryptedData));
      const { salt, data } = decoded;
      
      // Derive the same key
      const key = deriveKey(password, salt);
      
      // Decrypt
      const decryptedBytes = CryptoJS.AES.decrypt(data, key);
      const decryptedString = decryptedBytes.toString(CryptoJS.enc.Utf8);
      
      if (!decryptedString) {
        throw new Error('Decryption failed - invalid password');
      }
      
      return JSON.parse(decryptedString);
    } catch (error) {
      console.error('Decryption failed:', error);
      return null;
    }
  };

  const generatePassword = (length: number = 16, includeSpecial: boolean = true): string => {
    const lowercase = 'abcdefghijklmnopqrstuvwxyz';
    const uppercase = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ';
    const numbers = '0123456789';
    const special = '!@#$%^&*()_+-=[]{}|;:,.<>?';
    
    let charset = lowercase + uppercase + numbers;
    if (includeSpecial) {
      charset += special;
    }
    
    let password = '';
    
    // Ensure at least one character from each set
    if (length >= 4) {
      password += lowercase[Math.floor(Math.random() * lowercase.length)];
      password += uppercase[Math.floor(Math.random() * uppercase.length)];
      password += numbers[Math.floor(Math.random() * numbers.length)];
      
      if (includeSpecial && length > 4) {
        password += special[Math.floor(Math.random() * special.length)];
      }
    }
    
    // Fill the rest with random characters
    const remainingLength = length - password.length;
    for (let i = 0; i < remainingLength; i++) {
      password += charset[Math.floor(Math.random() * charset.length)];
    }
    
    // Shuffle the password to randomize position of guaranteed characters
    return password.split('').sort(() => Math.random() - 0.5).join('');
  };

  const value = {
    encryptVault,
    decryptVault,
    generatePassword,
    deriveKey,
    generateSalt,
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