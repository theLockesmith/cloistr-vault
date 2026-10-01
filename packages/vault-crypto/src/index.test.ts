import { describe, it, expect } from 'vitest';

import {
  ENVELOPE_VERSION,
  SCRYPT_PARAMS,
  randomBytes,
  generatePassword,
  passwordStrength,
  utf8ToBytes,
  bytesToUtf8,
  bytesToBase64,
  base64ToBytes,
  base32Decode,
  isLegacyVault,
  VaultFormatError,
} from './index';

describe('@cloistr/vault-crypto barrel export', () => {
  it('exports envelope constants', () => {
    expect(ENVELOPE_VERSION).toBe(2);
    expect(SCRYPT_PARAMS).toEqual({ N: 32768, r: 8, p: 1 });
  });

  it('exports encoding round-trip', () => {
    const text = 'hello vault';
    const bytes = utf8ToBytes(text);
    expect(bytesToUtf8(bytes)).toBe(text);

    const b64 = bytesToBase64(bytes);
    expect(base64ToBytes(b64)).toEqual(bytes);
  });

  it('exports randomBytes', () => {
    const a = randomBytes(32);
    const b = randomBytes(32);
    expect(a.length).toBe(32);
    expect(b.length).toBe(32);
    expect(a).not.toEqual(b);
  });

  it('exports generatePassword', () => {
    const pw = generatePassword(20, true);
    expect(pw.length).toBe(20);
  });

  it('exports passwordStrength', () => {
    const result = passwordStrength('abc');
    expect(result.score).toBeLessThanOrEqual(1);
    expect(result.label).toBeDefined();
  });

  it('exports base32Decode', () => {
    const decoded = base32Decode('JBSWY3DP');
    expect(bytesToUtf8(decoded)).toBe('Hello');
  });

  it('exports isLegacyVault', () => {
    expect(isLegacyVault('not-a-vault')).toBe(false);
  });

  it('exports VaultFormatError', () => {
    const err = new VaultFormatError('test');
    expect(err).toBeInstanceOf(Error);
    expect(err.message).toBe('test');
  });
});
