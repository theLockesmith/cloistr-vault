import CryptoJS from 'crypto-js';
import {
  UnlockedVault,
  VaultFormatError,
  enrollPasskey,
  enrolledPrfCredentialIds,
  newPrfSalt,
  prfSaltFor,
  revokePasskey,
  saveVault,
  serialize,
  unlockWithPassword,
  unlockWithPrf,
  wipe,
} from './vault';
import { deserializeEnvelope, isEnvelopeV2, SCRYPT_PARAMS } from './envelope';
import { isLegacyVault } from './legacy';
import { randomBytes } from './random';
import { base64ToBytes, bytesToBase64, encodeJson } from './encoding';

interface TestVault {
  entries: Array<{ id: string; name: string; secret: string }>;
  folders: string[];
}

const EMPTY: TestVault = { entries: [], folders: [] };

const sample = (): TestVault => ({
  entries: [
    { id: '1', name: 'email', secret: 'hunter2' },
    { id: '2', name: 'bank', secret: 'correct horse battery staple' },
  ],
  folders: ['personal'],
});

const PASSWORD = 'a-master-password';

/**
 * Produces a blob in the exact v1 format the shipped CryptoContext wrote, so
 * the migration path is tested against the real thing rather than against our
 * own reimplementation of it.
 */
function makeLegacyBlob(vault: unknown, password: string): string {
  const salt = CryptoJS.lib.WordArray.random(256 / 8).toString();
  const key = CryptoJS.PBKDF2(password, salt, {
    keySize: 256 / 32,
    iterations: 100000,
    hasher: CryptoJS.algo.SHA256,
  }).toString();
  const data = CryptoJS.AES.encrypt(JSON.stringify(vault), key).toString();
  return btoa(JSON.stringify({ salt, data }));
}

// scrypt at N=32768 is deliberately slow; these run it several times over.
jest.setTimeout(120_000);

describe('v2 envelope round-trip', () => {
  it('creates a vault when there is no stored blob', async () => {
    const unlocked = await unlockWithPassword(null, PASSWORD, EMPTY);
    expect(unlocked.data).toEqual(EMPTY);
    expect(unlocked.migrated).toBe(false);
    expect(isEnvelopeV2(serialize(unlocked))).toBe(true);
  });

  it('round-trips vault data through encrypt and decrypt', async () => {
    const created = await unlockWithPassword(null, PASSWORD, EMPTY);
    const saved = await saveVault(created, sample());

    const reopened = await unlockWithPassword<TestVault>(serialize(saved), PASSWORD, EMPTY);
    expect(reopened.data).toEqual(sample());
  });

  it('rejects the wrong password', async () => {
    const created = await unlockWithPassword(null, PASSWORD, EMPTY);
    const saved = await saveVault(created, sample());

    await expect(
      unlockWithPassword(serialize(saved), 'not-the-password', EMPTY)
    ).rejects.toThrow(/incorrect master password/i);
  });

  it('uses the documented scrypt parameters', async () => {
    const unlocked = await unlockWithPassword(null, PASSWORD, EMPTY);
    const envelope = deserializeEnvelope(serialize(unlocked));
    const wrap = envelope.wraps.find((w) => w.type === 'password');

    expect(wrap).toBeDefined();
    expect(wrap).toMatchObject({ kdf: 'scrypt', params: SCRYPT_PARAMS });
  });

  it('does not leak plaintext into the stored blob', async () => {
    const created = await unlockWithPassword(null, PASSWORD, EMPTY);
    const saved = await saveVault(created, sample());
    const blob = serialize(saved);

    expect(blob).not.toContain('hunter2');
    expect(atob(blob)).not.toContain('hunter2');
    expect(atob(blob)).not.toContain('correct horse');
  });

  it('produces different ciphertext for identical plaintext', async () => {
    const a = await unlockWithPassword(null, PASSWORD, EMPTY);
    const b = await unlockWithPassword(null, PASSWORD, EMPTY);
    expect(serialize(a)).not.toEqual(serialize(b));
  });

  it('saves without re-running the password KDF', async () => {
    // A save reuses the DEK, so the password wrap must come out byte-identical.
    const created = await unlockWithPassword(null, PASSWORD, EMPTY);
    const saved = await saveVault(created, sample());

    expect(saved.envelope.wraps).toEqual(created.envelope.wraps);
    expect(saved.envelope.ct).not.toEqual(created.envelope.ct);
  });
});

describe('authentication (GCM integrity)', () => {
  it('rejects a tampered body', async () => {
    const created = await unlockWithPassword(null, PASSWORD, EMPTY);
    const saved = await saveVault(created, sample());
    const envelope = deserializeEnvelope(serialize(saved));

    // Flip a bit in the body ciphertext.
    const ct = base64ToBytes(envelope.ct);
    ct[0] ^= 0x01;
    const tampered = encodeJson({ ...envelope, ct: bytesToBase64(ct) });

    await expect(unlockWithPassword(tampered, PASSWORD, EMPTY)).rejects.toThrow();
  });

  it('rejects a tampered wrap', async () => {
    const created = await unlockWithPassword(null, PASSWORD, EMPTY);
    const envelope = deserializeEnvelope(serialize(created));

    const wrap = envelope.wraps[0];
    const ct = base64ToBytes(wrap.ct);
    ct[0] ^= 0x01;
    const tampered = encodeJson({
      ...envelope,
      wraps: [{ ...wrap, ct: bytesToBase64(ct) }],
    });

    await expect(unlockWithPassword(tampered, PASSWORD, EMPTY)).rejects.toThrow();
  });
});

describe('v1 -> v2 migration', () => {
  it('recognises a real v1 blob', () => {
    const legacy = makeLegacyBlob(sample(), PASSWORD);
    expect(isLegacyVault(legacy)).toBe(true);
    expect(isEnvelopeV2(legacy)).toBe(false);
  });

  it('reads a v1 vault and re-keys it to v2', async () => {
    const legacy = makeLegacyBlob(sample(), PASSWORD);
    const unlocked = await unlockWithPassword<TestVault>(legacy, PASSWORD, EMPTY);

    expect(unlocked.data).toEqual(sample());
    expect(unlocked.migrated).toBe(true);
    expect(isEnvelopeV2(serialize(unlocked))).toBe(true);
  });

  it('produces a v2 blob that reopens with the same password', async () => {
    const legacy = makeLegacyBlob(sample(), PASSWORD);
    const migrated = await unlockWithPassword<TestVault>(legacy, PASSWORD, EMPTY);

    const reopened = await unlockWithPassword<TestVault>(serialize(migrated), PASSWORD, EMPTY);
    expect(reopened.data).toEqual(sample());
    expect(reopened.migrated).toBe(false);
  });

  it('rejects the wrong password against a v1 vault', async () => {
    const legacy = makeLegacyBlob(sample(), PASSWORD);
    await expect(unlockWithPassword(legacy, 'wrong', EMPTY)).rejects.toThrow(
      /incorrect master password/i
    );
  });

  it('rejects an unrecognised blob', async () => {
    await expect(unlockWithPassword('not-a-vault', PASSWORD, EMPTY)).rejects.toThrow(
      VaultFormatError
    );
  });
});

describe('passkey (PRF) enrolment', () => {
  const CRED = 'Y3JlZGVudGlhbC1pZA';

  async function unlockedWithPasskey(): Promise<{
    unlocked: UnlockedVault<TestVault>;
    prfSalt: Uint8Array;
    prfOutput: Uint8Array;
  }> {
    const created = await unlockWithPassword<TestVault>(null, PASSWORD, EMPTY);
    const saved = await saveVault(created, sample());
    const prfSalt = newPrfSalt();
    const prfOutput = randomBytes(32);
    const unlocked = await enrollPasskey(saved, CRED, prfSalt, prfOutput, 'YubiKey');
    return { unlocked, prfSalt, prfOutput };
  }

  it('unlocks with the PRF output after enrolment', async () => {
    const { unlocked, prfOutput } = await unlockedWithPasskey();

    const viaPasskey = await unlockWithPrf<TestVault>(serialize(unlocked), CRED, prfOutput);
    expect(viaPasskey.data).toEqual(sample());
  });

  it('keeps the master password working alongside the passkey', async () => {
    const { unlocked } = await unlockedWithPasskey();

    const viaPassword = await unlockWithPassword<TestVault>(serialize(unlocked), PASSWORD, EMPTY);
    expect(viaPassword.data).toEqual(sample());
  });

  it('yields the same DEK from both factors', async () => {
    const { unlocked, prfOutput } = await unlockedWithPasskey();
    const blob = serialize(unlocked);

    const viaPassword = await unlockWithPassword<TestVault>(blob, PASSWORD, EMPTY);
    const viaPasskey = await unlockWithPrf<TestVault>(blob, CRED, prfOutput);
    expect(Array.from(viaPasskey.dek)).toEqual(Array.from(viaPassword.dek));
  });

  it('rejects a wrong PRF output', async () => {
    const { unlocked } = await unlockedWithPasskey();
    await expect(
      unlockWithPrf(serialize(unlocked), CRED, randomBytes(32))
    ).rejects.toThrow(/passkey unlock failed/i);
  });

  it('rejects an unknown credential', async () => {
    const { unlocked, prfOutput } = await unlockedWithPasskey();
    await expect(
      unlockWithPrf(serialize(unlocked), 'some-other-credential', prfOutput)
    ).rejects.toThrow(/no passkey unlock enrolled/i);
  });

  it('does not re-encrypt the body when enrolling', async () => {
    const created = await unlockWithPassword<TestVault>(null, PASSWORD, EMPTY);
    const saved = await saveVault(created, sample());
    const enrolled = await enrollPasskey(saved, CRED, newPrfSalt(), randomBytes(32));

    expect(enrolled.envelope.ct).toEqual(saved.envelope.ct);
    expect(enrolled.envelope.nonce).toEqual(saved.envelope.nonce);
  });

  it('re-enrolling the same credential replaces rather than duplicates', async () => {
    const { unlocked } = await unlockedWithPasskey();
    const second = randomBytes(32);
    const reEnrolled = await enrollPasskey(unlocked, CRED, newPrfSalt(), second);

    expect(enrolledPrfCredentialIds(reEnrolled.envelope)).toEqual([CRED]);
    const viaPasskey = await unlockWithPrf<TestVault>(serialize(reEnrolled), CRED, second);
    expect(viaPasskey.data).toEqual(sample());
  });

  it('exposes the enrolment PRF salt for re-evaluation', async () => {
    const { unlocked, prfSalt } = await unlockedWithPasskey();
    expect(prfSaltFor(unlocked.envelope, CRED)).toEqual(prfSalt);
    expect(prfSaltFor(unlocked.envelope, 'unknown')).toBeNull();
  });

  it('revokes a passkey while leaving password access intact', async () => {
    const { unlocked, prfOutput } = await unlockedWithPasskey();
    const revoked = revokePasskey(unlocked, CRED);

    expect(enrolledPrfCredentialIds(revoked.envelope)).toEqual([]);
    await expect(unlockWithPrf(serialize(revoked), CRED, prfOutput)).rejects.toThrow();

    const viaPassword = await unlockWithPassword<TestVault>(serialize(revoked), PASSWORD, EMPTY);
    expect(viaPassword.data).toEqual(sample());
  });

  it('refuses passkey unlock on an unmigrated v1 vault', async () => {
    const legacy = makeLegacyBlob(sample(), PASSWORD);
    await expect(unlockWithPrf(legacy, CRED, randomBytes(32))).rejects.toThrow(VaultFormatError);
  });
});

describe('wipe', () => {
  it('zeroes the DEK', async () => {
    const unlocked = await unlockWithPassword(null, PASSWORD, EMPTY);
    expect(unlocked.dek.some((b) => b !== 0)).toBe(true);
    wipe(unlocked);
    expect(unlocked.dek.every((b) => b === 0)).toBe(true);
  });
});
