/**
 * RFC 6238 TOTP (Time-based One-Time Password) using the Web Crypto API.
 *
 * No external library required — every primitive used here (HMAC-SHA-1,
 * base32 decode, dynamic truncation) is implemented directly.
 */

/** Decodes a base32 string (RFC 4648, case-insensitive, padding optional). */
export function base32Decode(input: string): Uint8Array {
  const CHARS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  const s = input.toUpperCase().replace(/\s/g, '').replace(/=+$/, '');
  const out = new Uint8Array(Math.floor((s.length * 5) / 8));
  let bits = 0;
  let value = 0;
  let idx = 0;
  for (let i = 0; i < s.length; i++) {
    const c = CHARS.indexOf(s[i]);
    if (c === -1) {
      throw new Error(`Invalid base32 character '${s[i]}' at position ${i}`);
    }
    value = (value << 5) | c;
    bits += 5;
    if (bits >= 8) {
      out[idx++] = (value >>> (bits - 8)) & 0xff;
      bits -= 8;
    }
  }
  return out.slice(0, idx);
}

/**
 * Computes a 6-digit TOTP code for the given base32 secret.
 *
 * @param secret - Base32-encoded TOTP secret (as issued by the service)
 * @param time   - Unix timestamp in seconds (defaults to now)
 * @param period - Token period in seconds (default 30)
 * @param digits - Code length (default 6)
 */
export async function totp(
  secret: string,
  time: number = Math.floor(Date.now() / 1000),
  period: number = 30,
  digits: number = 6
): Promise<string> {
  const keyBytes = base32Decode(secret);
  const counter = Math.floor(time / period);

  // 8-byte big-endian counter
  const counterBuf = new ArrayBuffer(8);
  const view = new DataView(counterBuf);
  // JavaScript numbers are 64-bit floats — safe up to 2^53.
  view.setUint32(0, Math.floor(counter / 0x100000000), false);
  view.setUint32(4, counter >>> 0, false);

  // TS 5 tightened Uint8Array to Uint8Array<ArrayBuffer>; slicing produces a
  // fresh typed array with a plain ArrayBuffer, satisfying importKey's
  // BufferSource parameter without a cast.
  const cryptoKey = await crypto.subtle.importKey(
    'raw',
    keyBytes.slice() as Uint8Array<ArrayBuffer>,
    { name: 'HMAC', hash: 'SHA-1' },
    false,
    ['sign']
  );
  const mac = new Uint8Array(await crypto.subtle.sign('HMAC', cryptoKey, counterBuf));

  // Dynamic truncation
  const offset = mac[mac.length - 1] & 0x0f;
  const code =
    (((mac[offset] & 0x7f) << 24) |
      ((mac[offset + 1] & 0xff) << 16) |
      ((mac[offset + 2] & 0xff) << 8) |
      (mac[offset + 3] & 0xff)) %
    Math.pow(10, digits);

  return String(code).padStart(digits, '0');
}

/**
 * Returns the number of seconds remaining in the current TOTP window.
 */
export function totpSecondsRemaining(period: number = 30): number {
  return period - (Math.floor(Date.now() / 1000) % period);
}
