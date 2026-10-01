/**
 * Byte/string conversions used by the vault envelope format.
 *
 * `btoa`/`atob` operate on "binary strings" (one code unit per byte) and throw
 * on any code point above U+00FF, so they cannot be handed UTF-8 text or a
 * Uint8Array directly. Everything crossing that boundary goes through here.
 */

export function utf8ToBytes(text: string): Uint8Array {
  return new TextEncoder().encode(text);
}

export function bytesToUtf8(bytes: Uint8Array): string {
  return new TextDecoder().decode(bytes);
}

export function bytesToBase64(bytes: Uint8Array): string {
  let binary = '';
  // Chunked to keep the argument list well under the engine's spread limit;
  // a whole vault passed at once would blow the call stack.
  const CHUNK = 0x8000;
  for (let i = 0; i < bytes.length; i += CHUNK) {
    binary += String.fromCharCode(...bytes.subarray(i, i + CHUNK));
  }
  return btoa(binary);
}

export function base64ToBytes(base64: string): Uint8Array {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

/** Base64url (RFC 4648 §5) — the encoding WebAuthn uses for credential IDs. */
export function bytesToBase64Url(bytes: Uint8Array): string {
  return bytesToBase64(bytes).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

export function base64UrlToBytes(base64url: string): Uint8Array {
  const padded = base64url
    .replace(/-/g, '+')
    .replace(/_/g, '/')
    .padEnd(Math.ceil(base64url.length / 4) * 4, '=');
  return base64ToBytes(padded);
}

/** Encodes an object as base64(JSON) — the on-the-wire form of a vault blob. */
export function encodeJson(value: unknown): string {
  return bytesToBase64(utf8ToBytes(JSON.stringify(value)));
}

/** Inverse of `encodeJson`. Returns null if the input is not base64 JSON. */
export function decodeJson(encoded: string): unknown | null {
  try {
    return JSON.parse(bytesToUtf8(base64ToBytes(encoded)));
  } catch {
    return null;
  }
}
