// Vitest setup, loaded before each test file via vite.config.ts setupFiles.

import '@testing-library/jest-dom';
import { webcrypto } from 'crypto';
import { TextDecoder, TextEncoder } from 'util';

// jsdom omits the Encoding API too. Node's implementations are spec-compatible.
if (typeof globalThis.TextEncoder === 'undefined') {
  globalThis.TextEncoder = TextEncoder as typeof globalThis.TextEncoder;
}
if (typeof globalThis.TextDecoder === 'undefined') {
  globalThis.TextDecoder = TextDecoder as typeof globalThis.TextDecoder;
}

// jsdom does not install a WebCrypto implementation on its global object, so
// `crypto.getRandomValues` and `crypto.subtle` are undefined under test even
// though every browser we target provides them. Bridge Node's implementation
// across; it is the same WebCrypto API surface.
if (typeof globalThis.crypto === 'undefined') {
  Object.defineProperty(globalThis, 'crypto', {
    value: webcrypto,
    configurable: true,
    writable: true,
  });
} else {
  if (typeof globalThis.crypto.getRandomValues !== 'function') {
    (globalThis.crypto as any).getRandomValues = (webcrypto as any).getRandomValues.bind(webcrypto);
  }
  if (typeof globalThis.crypto.subtle === 'undefined') {
    Object.defineProperty(globalThis.crypto, 'subtle', {
      value: (webcrypto as any).subtle,
      configurable: true,
    });
  }
}
