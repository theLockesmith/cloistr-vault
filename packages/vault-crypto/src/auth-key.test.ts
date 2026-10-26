import { describe, expect, it } from 'vitest';
import { deriveAuthKey, normalizeEmail } from './auth-key';

// Vectors computed independently with Python's hashlib.scrypt:
//   salt = sha256(b'cloistr-vault/auth/v1|' + email)
//   scrypt(password_utf8, salt, n=32768, r=8, p=1, dklen=32).hex()
describe('deriveAuthKey', () => {
  it('matches an independent scrypt implementation', async () => {
    expect(await deriveAuthKey('alice@example.com', 'correct horse battery staple')).toBe(
      '3bbf29e8afd1f96650f270e8f2683f6d9837c07e3960aaa403f731912bfbfd43'
    );
    expect(await deriveAuthKey('bob@example.com', 'pässwörd ✓')).toBe(
      'eb67f639bda4b725cd34e8d9786aa036673d09b1d84161e04f2b1b97b9613f85'
    );
  });

  it('salts by the normalised email, so case and spaces do not change the key', async () => {
    expect(await deriveAuthKey('  Alice@Example.COM ', 'correct horse battery staple')).toBe(
      '3bbf29e8afd1f96650f270e8f2683f6d9837c07e3960aaa403f731912bfbfd43'
    );
  });

  it('never returns the password or anything containing it', async () => {
    const password = 'correct horse battery staple';
    const key = await deriveAuthKey('alice@example.com', password);
    expect(key).toMatch(/^[0-9a-f]{64}$/);
    expect(key).not.toContain(password);
  });

  it('differs per email for the same password', async () => {
    const a = await deriveAuthKey('alice@example.com', 'same password');
    const b = await deriveAuthKey('bob@example.com', 'same password');
    expect(a).not.toBe(b);
  });
});

describe('normalizeEmail', () => {
  it('trims and lowercases', () => {
    expect(normalizeEmail('  Alice@Example.COM ')).toBe('alice@example.com');
  });
});
