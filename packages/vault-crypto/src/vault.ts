/**
 * The vault crypto entry point. UI code should use only this module.
 *
 * Handles format detection and the v1 -> v2 migration, so callers never need to
 * know which format a stored blob is in.
 */

import {
  VaultEnvelopeV2,
  addPrfWrap,
  createEnvelope,
  decryptBody,
  deserializeEnvelope,
  encryptBody,
  enrolledPrfCredentialIds,
  isEnvelopeV2,
  newPrfSalt,
  prfSaltFor,
  removePrfWrap,
  serializeEnvelope,
  unwrapWithPassword,
  unwrapWithPrf,
} from './envelope';
import { decryptLegacyVault, isLegacyVault } from './legacy';

export type { VaultEnvelopeV2, KeyWrap, PasswordWrap, PrfWrap } from './envelope';
export { ENVELOPE_VERSION, SCRYPT_PARAMS } from './envelope';
export { newPrfSalt, prfSaltFor, enrolledPrfCredentialIds };

/**
 * An unlocked vault.
 *
 * `dek` is live key material: hold it only for the duration of a session and
 * drop it on lock. It is what lets a save skip scrypt entirely.
 */
export interface UnlockedVault<T> {
  data: T;
  envelope: VaultEnvelopeV2;
  dek: Uint8Array;
  /** True when the stored blob was v1 and has been rewritten as v2 in memory. */
  migrated: boolean;
}

export class VaultFormatError extends Error {}

/**
 * Unlocks a stored blob with the master password, migrating v1 to v2 in the
 * process.
 *
 * A migrated result is not yet persisted — the caller must write
 * `serialize(result)` back to the server. Until it does, the vault stays
 * readable in its original v1 form, so an interrupted migration loses nothing.
 */
export async function unlockWithPassword<T>(
  blob: string | null | undefined,
  password: string,
  emptyVault: T
): Promise<UnlockedVault<T>> {
  // No vault stored yet — create one.
  if (!blob) {
    const { envelope, dek } = await createEnvelope(emptyVault, password);
    return { data: emptyVault, envelope, dek, migrated: false };
  }

  if (isEnvelopeV2(blob)) {
    const envelope = deserializeEnvelope(blob);
    const dek = await unwrapWithPassword(envelope, password);
    const data = await decryptBody<T>(envelope, dek);
    return { data, envelope, dek, migrated: false };
  }

  if (isLegacyVault(blob)) {
    const data = decryptLegacyVault<T>(blob, password);
    if (data === null) {
      throw new Error('Incorrect master password');
    }
    // Re-key onto v2: fresh DEK, fresh scrypt salt, authenticated body.
    const { envelope, dek } = await createEnvelope(data, password);
    return { data, envelope, dek, migrated: true };
  }

  throw new VaultFormatError('Unrecognised vault format');
}

/**
 * Unlocks a v2 vault with a passkey PRF output.
 *
 * v1 vaults cannot be opened this way — there is no wrap to unwrap. Those must
 * be unlocked with the password once, which migrates them, before a passkey can
 * be enrolled.
 */
export async function unlockWithPrf<T>(
  blob: string,
  credentialId: string,
  prfOutput: Uint8Array
): Promise<UnlockedVault<T>> {
  if (!isEnvelopeV2(blob)) {
    throw new VaultFormatError(
      'This vault predates passkey unlock — sign in with your master password once to upgrade it'
    );
  }

  const envelope = deserializeEnvelope(blob);
  const dek = await unwrapWithPrf(envelope, credentialId, prfOutput);
  const data = await decryptBody<T>(envelope, dek);
  return { data, envelope, dek, migrated: false };
}

/**
 * Seals a starter vault at registration time, before any session exists.
 *
 * Returns the blob to hand to the register endpoint. The DEK is discarded —
 * the user's first unlock re-derives it from the password.
 */
export async function createInitialEnvelope<T>(vault: T, password: string): Promise<string> {
  const { envelope } = await createEnvelope(vault, password);
  return serializeEnvelope(envelope);
}

/** Re-encrypts the body under the existing DEK. Does not re-run scrypt. */
export async function saveVault<T>(
  unlocked: UnlockedVault<T>,
  data: T
): Promise<UnlockedVault<T>> {
  const envelope = await encryptBody(unlocked.envelope, unlocked.dek, data);
  return { ...unlocked, data, envelope, migrated: false };
}

/**
 * Reads the passkey enrolments out of a stored blob without unlocking it.
 *
 * Returns credential ID -> the PRF salt it was enrolled with, which is exactly
 * what an unlock assertion needs. Empty for a v1 or unparseable blob, so the UI
 * can simply check for keys before offering passkey unlock.
 */
export function passkeyUnlockSalts(blob: string | null | undefined): Record<string, Uint8Array> {
  if (!blob || !isEnvelopeV2(blob)) {
    return {};
  }

  const envelope = deserializeEnvelope(blob);
  const salts: Record<string, Uint8Array> = {};
  for (const credentialId of enrolledPrfCredentialIds(envelope)) {
    const salt = prfSaltFor(envelope, credentialId);
    if (salt) {
      salts[credentialId] = salt;
    }
  }
  return salts;
}

/** Enrols a passkey as an additional unlock factor. */
export async function enrollPasskey<T>(
  unlocked: UnlockedVault<T>,
  credentialId: string,
  prfSalt: Uint8Array,
  prfOutput: Uint8Array,
  label?: string
): Promise<UnlockedVault<T>> {
  const envelope = await addPrfWrap(
    unlocked.envelope,
    unlocked.dek,
    credentialId,
    prfSalt,
    prfOutput,
    label
  );
  return { ...unlocked, envelope, migrated: false };
}

/** Removes a passkey unlock factor. The password wrap always remains. */
export function revokePasskey<T>(unlocked: UnlockedVault<T>, credentialId: string): UnlockedVault<T> {
  return { ...unlocked, envelope: removePrfWrap(unlocked.envelope, credentialId), migrated: false };
}

/** The blob to persist for an unlocked vault. */
export function serialize<T>(unlocked: UnlockedVault<T>): string {
  return serializeEnvelope(unlocked.envelope);
}

/** Best-effort zeroing of the DEK when locking. */
export function wipe<T>(unlocked: UnlockedVault<T>): void {
  unlocked.dek.fill(0);
}
