/**
 * Vault envelope format v2.
 *
 * The v1 format derived a key straight from the master password and encrypted
 * the whole vault under it. That made the password the only possible unlock
 * path: any second factor would have had to reproduce the same key, which a
 * WebAuthn authenticator cannot do.
 *
 * v2 splits the two concerns:
 *
 *   DEK  a random 256-bit data encryption key. Encrypts the vault body, and
 *        never changes for the life of the vault.
 *   KEK  a key encryption key derived from one unlock factor. Encrypts (wraps)
 *        a copy of the DEK.
 *
 * An envelope carries one wrap per enrolled factor. Adding a passkey appends a
 * wrap; it does not touch the body. Saving the vault rewrites the body under
 * the existing DEK; it does not touch the wraps. Unlocking means deriving one
 * KEK, unwrapping the DEK with it, and decrypting the body.
 *
 * Algorithms match what docs/security.md has always claimed: scrypt
 * (N=32768, r=8, p=1) for password-derived KEKs, AES-256-GCM everywhere.
 */

import { scryptAsync } from '@noble/hashes/scrypt.js';
import { hkdf } from '@noble/hashes/hkdf.js';
import { sha256 } from '@noble/hashes/sha2.js';
import {
  base64ToBytes,
  bytesToBase64,
  bytesToUtf8,
  decodeJson,
  encodeJson,
  utf8ToBytes,
} from './encoding';
import { randomBytes } from './random';

export const ENVELOPE_VERSION = 2;

/** Matches backend/internal/crypto/crypto.go and docs/security.md. */
export const SCRYPT_PARAMS = { N: 32768, r: 8, p: 1 } as const;

const KEY_BYTES = 32; // AES-256
const NONCE_BYTES = 12; // GCM standard nonce
const SALT_BYTES = 32;

export interface ScryptParams {
  N: number;
  r: number;
  p: number;
}

/** A DEK wrapped under a password-derived KEK. */
export interface PasswordWrap {
  type: 'password';
  kdf: 'scrypt';
  params: ScryptParams;
  salt: string;
  nonce: string;
  ct: string;
}

/** A DEK wrapped under a KEK derived from a WebAuthn PRF output. */
export interface PrfWrap {
  type: 'prf';
  credentialId: string;
  prfSalt: string;
  nonce: string;
  ct: string;
  label?: string;
}

export type KeyWrap = PasswordWrap | PrfWrap;

export interface VaultEnvelopeV2 {
  v: typeof ENVELOPE_VERSION;
  wraps: KeyWrap[];
  nonce: string;
  ct: string;
}

/**
 * Additional authenticated data. GCM binds the ciphertext to these bytes, so a
 * wrap cannot be replayed as the body, and a password wrap cannot be presented
 * as a PRF wrap.
 */
const AAD_BODY = utf8ToBytes('cloistr-vault/v2/body');
const aadForWrap = (type: KeyWrap['type']) => utf8ToBytes(`cloistr-vault/v2/wrap/${type}`);

async function importAesKey(raw: Uint8Array): Promise<CryptoKey> {
  return crypto.subtle.importKey('raw', raw as BufferSource, 'AES-GCM', false, [
    'encrypt',
    'decrypt',
  ]);
}

async function aesGcmEncrypt(
  key: CryptoKey,
  plaintext: Uint8Array,
  aad: Uint8Array
): Promise<{ nonce: string; ct: string }> {
  const nonce = randomBytes(NONCE_BYTES);
  const ct = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv: nonce as BufferSource, additionalData: aad as BufferSource },
    key,
    plaintext as BufferSource
  );
  return { nonce: bytesToBase64(nonce), ct: bytesToBase64(new Uint8Array(ct)) };
}

async function aesGcmDecrypt(
  key: CryptoKey,
  nonce: string,
  ct: string,
  aad: Uint8Array
): Promise<Uint8Array> {
  const plaintext = await crypto.subtle.decrypt(
    {
      name: 'AES-GCM',
      iv: base64ToBytes(nonce) as BufferSource,
      additionalData: aad as BufferSource,
    },
    key,
    base64ToBytes(ct) as BufferSource
  );
  return new Uint8Array(plaintext);
}

/**
 * Derives a KEK from the master password.
 *
 * scryptAsync yields to the event loop between blocks; at N=32768 this costs
 * ~32 MiB and takes on the order of a second, which is the point of it.
 */
async function derivePasswordKek(
  password: string,
  salt: Uint8Array,
  params: ScryptParams
): Promise<CryptoKey> {
  const raw = await scryptAsync(utf8ToBytes(password), salt, {
    N: params.N,
    r: params.r,
    p: params.p,
    dkLen: KEY_BYTES,
  });
  return importAesKey(raw);
}

/**
 * Derives a KEK from a WebAuthn PRF output.
 *
 * No scrypt here: the PRF result is already 32 uniformly random bytes from the
 * authenticator's HMAC secret, so stretching it buys nothing. HKDF only
 * domain-separates it from any other use of the same PRF output.
 */
async function derivePrfKek(prfOutput: Uint8Array): Promise<CryptoKey> {
  const raw = hkdf(sha256, prfOutput, undefined, utf8ToBytes('cloistr-vault/v2/prf-kek'), KEY_BYTES);
  return importAesKey(raw);
}

/** True if `blob` is a v2 envelope. */
export function isEnvelopeV2(blob: string): boolean {
  const parsed = decodeJson(blob) as Partial<VaultEnvelopeV2> | null;
  return !!parsed && parsed.v === ENVELOPE_VERSION && Array.isArray(parsed.wraps);
}

function parseEnvelope(blob: string): VaultEnvelopeV2 {
  const parsed = decodeJson(blob) as VaultEnvelopeV2 | null;
  if (!parsed || parsed.v !== ENVELOPE_VERSION || !Array.isArray(parsed.wraps)) {
    throw new Error('Not a v2 vault envelope');
  }
  return parsed;
}

export function serializeEnvelope(envelope: VaultEnvelopeV2): string {
  return encodeJson(envelope);
}

export function deserializeEnvelope(blob: string): VaultEnvelopeV2 {
  return parseEnvelope(blob);
}

/**
 * Creates a fresh envelope: new DEK, one password wrap, body encrypted.
 *
 * Returns the DEK alongside the envelope so the caller can keep the vault
 * unlocked (and enrol further factors) without re-running scrypt.
 */
export async function createEnvelope<T>(
  vault: T,
  password: string
): Promise<{ envelope: VaultEnvelopeV2; dek: Uint8Array }> {
  const dek = randomBytes(KEY_BYTES);
  const body = await aesGcmEncrypt(
    await importAesKey(dek),
    utf8ToBytes(JSON.stringify(vault)),
    AAD_BODY
  );
  const wrap = await buildPasswordWrap(dek, password);

  return {
    envelope: { v: ENVELOPE_VERSION, wraps: [wrap], nonce: body.nonce, ct: body.ct },
    dek,
  };
}

async function buildPasswordWrap(dek: Uint8Array, password: string): Promise<PasswordWrap> {
  const salt = randomBytes(SALT_BYTES);
  const kek = await derivePasswordKek(password, salt, SCRYPT_PARAMS);
  const { nonce, ct } = await aesGcmEncrypt(kek, dek, aadForWrap('password'));
  return {
    type: 'password',
    kdf: 'scrypt',
    params: { ...SCRYPT_PARAMS },
    salt: bytesToBase64(salt),
    nonce,
    ct,
  };
}

/**
 * Recovers the DEK using the master password.
 *
 * Tries every password wrap: a vault normally has one, but tolerating several
 * means a password change can be staged without a flag day.
 */
export async function unwrapWithPassword(
  envelope: VaultEnvelopeV2,
  password: string
): Promise<Uint8Array> {
  const wraps = envelope.wraps.filter((w): w is PasswordWrap => w.type === 'password');
  if (wraps.length === 0) {
    throw new Error('This vault has no password unlock enrolled');
  }

  for (const wrap of wraps) {
    try {
      const kek = await derivePasswordKek(password, base64ToBytes(wrap.salt), wrap.params);
      return await aesGcmDecrypt(kek, wrap.nonce, wrap.ct, aadForWrap('password'));
    } catch {
      // Wrong password for this wrap, or a tampered wrap. Try the next.
    }
  }
  throw new Error('Incorrect master password');
}

/** Recovers the DEK from a WebAuthn PRF output for a given credential. */
export async function unwrapWithPrf(
  envelope: VaultEnvelopeV2,
  credentialId: string,
  prfOutput: Uint8Array
): Promise<Uint8Array> {
  const wrap = envelope.wraps.find(
    (w): w is PrfWrap => w.type === 'prf' && w.credentialId === credentialId
  );
  if (!wrap) {
    throw new Error('No passkey unlock enrolled for this credential');
  }

  const kek = await derivePrfKek(prfOutput);
  try {
    return await aesGcmDecrypt(kek, wrap.nonce, wrap.ct, aadForWrap('prf'));
  } catch {
    throw new Error('Passkey unlock failed — the authenticator returned an unexpected secret');
  }
}

/** The PRF salt a credential's wrap was enrolled with, needed to re-evaluate it. */
export function prfSaltFor(envelope: VaultEnvelopeV2, credentialId: string): Uint8Array | null {
  const wrap = envelope.wraps.find(
    (w): w is PrfWrap => w.type === 'prf' && w.credentialId === credentialId
  );
  return wrap ? base64ToBytes(wrap.prfSalt) : null;
}

export function enrolledPrfCredentialIds(envelope: VaultEnvelopeV2): string[] {
  return envelope.wraps.filter((w): w is PrfWrap => w.type === 'prf').map((w) => w.credentialId);
}

/** Decrypts the vault body with a recovered DEK. */
export async function decryptBody<T>(envelope: VaultEnvelopeV2, dek: Uint8Array): Promise<T> {
  const plaintext = await aesGcmDecrypt(
    await importAesKey(dek),
    envelope.nonce,
    envelope.ct,
    AAD_BODY
  );
  return JSON.parse(bytesToUtf8(plaintext)) as T;
}

/** Re-encrypts the body under the existing DEK, leaving all wraps intact. */
export async function encryptBody<T>(
  envelope: VaultEnvelopeV2,
  dek: Uint8Array,
  vault: T
): Promise<VaultEnvelopeV2> {
  const body = await aesGcmEncrypt(
    await importAesKey(dek),
    utf8ToBytes(JSON.stringify(vault)),
    AAD_BODY
  );
  return { ...envelope, nonce: body.nonce, ct: body.ct };
}

/**
 * Adds a passkey unlock factor, replacing any existing wrap for the same
 * credential so re-enrolling is idempotent.
 */
export async function addPrfWrap(
  envelope: VaultEnvelopeV2,
  dek: Uint8Array,
  credentialId: string,
  prfSalt: Uint8Array,
  prfOutput: Uint8Array,
  label?: string
): Promise<VaultEnvelopeV2> {
  const kek = await derivePrfKek(prfOutput);
  const { nonce, ct } = await aesGcmEncrypt(kek, dek, aadForWrap('prf'));
  const wrap: PrfWrap = {
    type: 'prf',
    credentialId,
    prfSalt: bytesToBase64(prfSalt),
    nonce,
    ct,
    ...(label ? { label } : {}),
  };

  return {
    ...envelope,
    wraps: [...envelope.wraps.filter((w) => !(w.type === 'prf' && w.credentialId === credentialId)), wrap],
  };
}

/**
 * Removes a passkey unlock factor.
 *
 * Refuses to remove the last remaining wrap of any kind — an envelope with no
 * wraps is an unrecoverable vault.
 */
export function removePrfWrap(envelope: VaultEnvelopeV2, credentialId: string): VaultEnvelopeV2 {
  const remaining = envelope.wraps.filter(
    (w) => !(w.type === 'prf' && w.credentialId === credentialId)
  );
  if (remaining.length === 0) {
    throw new Error('Refusing to remove the only unlock factor — the vault would be unrecoverable');
  }
  return { ...envelope, wraps: remaining };
}

/** Generates a fresh PRF evaluation salt for a new enrolment. */
export function newPrfSalt(): Uint8Array {
  return randomBytes(SALT_BYTES);
}
