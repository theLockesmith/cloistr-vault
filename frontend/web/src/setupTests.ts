// Jest setup, auto-loaded by react-scripts before each test file.

import '@testing-library/jest-dom';
import { webcrypto } from 'crypto';

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
