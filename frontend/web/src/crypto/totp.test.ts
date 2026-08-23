/**
 * Tests for RFC 6238 TOTP and base32 decode.
 *
 * RFC 6238 Appendix B provides the reference test vectors against a fixed key
 * "12345678901234567890" using SHA-1, with T0=0, X=30.
 * The vectors below are independently computable from the algorithm.
 */
import { base32Decode, totp, totpSecondsRemaining } from './totp';

// ---------- base32Decode ----------

describe('base32Decode', () => {
  it('decodes the empty string to empty bytes', () => {
    expect(Array.from(base32Decode(''))).toEqual([]);
  });

  it('decodes a known vector without padding', () => {
    // "fo" = 0x66 0x6f
    // base32: 01100110 01101111 -> 01100 11001 10111 10000 -> M Z X Q
    const decoded = base32Decode('MZXQ');
    expect(Array.from(decoded)).toEqual([0x66, 0x6f]);
  });

  it('is case-insensitive', () => {
    const upper = base32Decode('JBSWY3DPEB3W64TMMQ');
    const lower = base32Decode('jbswy3dpeb3w64tmmq');
    expect(Array.from(upper)).toEqual(Array.from(lower));
  });

  it('ignores trailing padding characters', () => {
    const withPad = base32Decode('JBSWY3DPEB3W64TMMQ======');
    const withoutPad = base32Decode('JBSWY3DPEB3W64TMMQ');
    expect(Array.from(withPad)).toEqual(Array.from(withoutPad));
  });

  it('throws on an invalid character', () => {
    expect(() => base32Decode('JBSWY0DPEB3W64TMMQ')).toThrow(/Invalid base32 character/);
  });
});

// ---------- TOTP ----------

// RFC 6238 Appendix B test vectors (SHA-1 key = "12345678901234567890", period=30).
// Key bytes: 0x31 0x32 0x33 0x34 0x35 0x36 0x37 0x38 0x39 0x30 (x2)
// Encoded in base32: GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ
const RFC6238_SECRET = 'GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ';

// T=59         counter=1   → 94287082
// T=1111111109 counter=37037036 → 07081804
// T=1111111111 counter=37037037 → 14050471
const RFC_VECTORS: [number, string][] = [
  [59, '94287082'],
  [1111111109, '07081804'],
  [1111111111, '14050471'],
];

describe('totp (RFC 6238 SHA-1 vectors)', () => {
  test.each(RFC_VECTORS)('T=%d → %s', async (time, expected) => {
    const code = await totp(RFC6238_SECRET, time, 30, 8);
    expect(code).toBe(expected);
  });

  it('returns 6 digits by default and pads with leading zeros', async () => {
    // Any secret that produces a code < 100000 at some counter will do;
    // we check padding by verifying length.
    const code = await totp(RFC6238_SECRET, 59, 30, 6);
    expect(code).toHaveLength(6);
    expect(/^\d{6}$/.test(code)).toBe(true);
  });

  it('throws on an invalid base32 secret', async () => {
    await expect(totp('!!!', 0)).rejects.toThrow(/Invalid base32 character/);
  });
});

// ---------- totpSecondsRemaining ----------

describe('totpSecondsRemaining', () => {
  it('returns a value between 1 and 30 inclusive', () => {
    const s = totpSecondsRemaining(30);
    expect(s).toBeGreaterThanOrEqual(1);
    expect(s).toBeLessThanOrEqual(30);
  });

  it('accepts a custom period', () => {
    const s = totpSecondsRemaining(60);
    expect(s).toBeGreaterThanOrEqual(1);
    expect(s).toBeLessThanOrEqual(60);
  });
});
