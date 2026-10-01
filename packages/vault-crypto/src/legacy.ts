/**
 * Read-only support for the v1 vault format.
 *
 * v1 is what shipped, not what was documented. docs/security.md described
 * AES-256-GCM with scrypt; the implementation was:
 *
 *   key = PBKDF2-SHA256(password, salt, 100_000).toString()   // hex string
 *   blob = CryptoJS.AES.encrypt(json, key)                    // key as passphrase
 *
 * Passing a *string* as CryptoJS's key makes it a passphrase, not raw key
 * material: CryptoJS then runs OpenSSL's EVP_BytesToKey (single-round MD5,
 * random 8-byte salt) over that hex string to produce the actual key and IV,
 * and encrypts with AES-256-CBC. So v1 vaults are unauthenticated CBC — no
 * integrity protection at all — with a 1-round MD5 final KDF step.
 *
 * This module exists only to read those vaults once so they can be rewritten
 * as v2. There is deliberately no v1 writer. Delete this file once no vault in
 * the wild is still on v1.
 */

import CryptoJS from 'crypto-js';

interface LegacyBlob {
  salt: string;
  data: string;
}

/** True if `blob` looks like a v1 vault. */
export function isLegacyVault(blob: string): boolean {
  return parseLegacyBlob(blob) !== null;
}

function parseLegacyBlob(blob: string): LegacyBlob | null {
  try {
    const decoded = JSON.parse(atob(blob));
    if (
      decoded &&
      typeof decoded.salt === 'string' &&
      typeof decoded.data === 'string' &&
      decoded.v === undefined
    ) {
      return decoded as LegacyBlob;
    }
    return null;
  } catch {
    return null;
  }
}

/**
 * Decrypts a v1 vault. Returns null when the password is wrong.
 *
 * CBC has no authentication tag, so a wrong key does not fail cleanly — it
 * yields garbage. The only available signal is that the plaintext fails to
 * decode as UTF-8 or to parse as JSON, which is why this is best-effort.
 */
export function decryptLegacyVault<T>(blob: string, password: string): T | null {
  const parsed = parseLegacyBlob(blob);
  if (!parsed) {
    return null;
  }

  try {
    const key = CryptoJS.PBKDF2(password, parsed.salt, {
      keySize: 256 / 32,
      iterations: 100000,
      hasher: CryptoJS.algo.SHA256,
    }).toString();

    const decrypted = CryptoJS.AES.decrypt(parsed.data, key).toString(CryptoJS.enc.Utf8);
    if (!decrypted) {
      return null;
    }
    return JSON.parse(decrypted) as T;
  } catch {
    return null;
  }
}
