import { scryptAsync } from '@noble/hashes/scrypt.js';
import { sha256 } from '@noble/hashes/sha2.js';
import { bytesToHex } from '@noble/hashes/utils.js';
import { utf8ToBytes } from './encoding';
import { SCRYPT_PARAMS } from './envelope';

/**
 * Domain tag for the login key. Changing it changes every account's auth key,
 * so it is versioned rather than edited.
 */
const AUTH_SALT_PREFIX = 'cloistr-vault/auth/v1|';

/** The email form the auth-key salt is computed over. */
export function normalizeEmail(email: string): string {
  return email.trim().toLowerCase();
}

/**
 * Derives the value an email account signs in with, so the master password
 * itself never leaves the device.
 *
 * scrypt(password, sha256(prefix + email)), same cost as the vault KEK. The
 * KEK uses a random per-envelope salt instead, so this key and the KEK are
 * independent: knowing the auth key gives the server nothing it can decrypt
 * with, and recovering the password from it costs a full scrypt per guess.
 *
 * The salt comes from the email so that it is known before sign-in without a
 * lookup that would reveal whether an account exists. The consequence: an
 * email change must re-derive this key and replace the server's verifier in
 * the same step.
 *
 * Returns 64 lowercase hex characters.
 */
export async function deriveAuthKey(email: string, password: string): Promise<string> {
  const salt = sha256(utf8ToBytes(AUTH_SALT_PREFIX + normalizeEmail(email)));
  const key = await scryptAsync(utf8ToBytes(password), salt, {
    N: SCRYPT_PARAMS.N,
    r: SCRYPT_PARAMS.r,
    p: SCRYPT_PARAMS.p,
    dkLen: 32,
  });
  return bytesToHex(key);
}
