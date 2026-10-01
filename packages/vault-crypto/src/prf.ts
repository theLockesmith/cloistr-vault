/**
 * WebAuthn PRF extension — the bridge between an authenticator and a vault key.
 *
 * The PRF extension (backed by the CTAP2 hmac-secret extension) lets a relying
 * party ask an authenticator to evaluate HMAC over a caller-supplied salt using
 * a secret that never leaves the device. The same credential and the same salt
 * always yield the same 32 bytes, and nothing else can produce them.
 *
 * That is what makes passkey vault unlock possible at all. A normal WebAuthn
 * assertion only proves possession — the signature is over a server challenge
 * and yields no reusable key material. PRF gives us a stable secret we can
 * derive a KEK from, so the vault's DEK can be wrapped for a passkey without
 * the server ever seeing anything.
 *
 * Support is not universal: it needs both a browser that implements the
 * extension and an authenticator that implements hmac-secret. Everything here
 * degrades to a clear error so the caller can fall back to the master password.
 */

import { base64UrlToBytes, bytesToBase64Url } from './encoding';

/** Raised when PRF is unavailable, as opposed to the user cancelling. */
export class PrfUnsupportedError extends Error {}

/** Raised when the user dismisses or cancels the authenticator prompt. */
export class PrfCancelledError extends Error {}

interface PrfExtensionResults {
  enabled?: boolean;
  results?: { first?: ArrayBuffer; second?: ArrayBuffer };
}

// Declared as a standalone type rather than extending
// AuthenticationExtensionsClientOutputs because the TS 5 DOM types define
// AuthenticationExtensionsPRFValues.first as required BufferSource, which
// conflicts with our looser optional ArrayBuffer.  We only ever use this as
// an `as` cast from getClientExtensionResults(), so no extends is needed.
interface ExtensionResultsWithPrf {
  prf?: PrfExtensionResults;
}

/** True if this browser exposes WebAuthn at all. */
export function isWebAuthnAvailable(): boolean {
  return typeof window !== 'undefined' && !!window.PublicKeyCredential;
}

function isCancellation(error: unknown): boolean {
  return (
    error instanceof DOMException &&
    (error.name === 'NotAllowedError' || error.name === 'AbortError')
  );
}

/**
 * Runs an assertion against one or more credentials, asking the authenticator
 * to evaluate the PRF over `salt`.
 *
 * `evalByCredential` is used when more than one credential is offered, since
 * each enrolment carries its own salt and we cannot know in advance which
 * credential the user will reach for. With a single credential the simpler
 * `eval` form is used — Safari has historically been stricter about
 * evalByCredential requiring a non-empty allowCredentials list.
 *
 * Returns the PRF output together with the credential that produced it.
 */
export async function evaluatePrf(
  saltsByCredentialId: Record<string, Uint8Array>,
  options: { challenge?: Uint8Array; signal?: AbortSignal } = {}
): Promise<{ credentialId: string; prfOutput: Uint8Array }> {
  if (!isWebAuthnAvailable()) {
    throw new PrfUnsupportedError('This browser does not support WebAuthn');
  }

  const credentialIds = Object.keys(saltsByCredentialId);
  if (credentialIds.length === 0) {
    throw new PrfUnsupportedError('No passkey is enrolled for vault unlock');
  }

  // The assertion signature is discarded — we only want the PRF output — so
  // this challenge is not a server challenge and carries no replay meaning.
  const challenge = options.challenge ?? crypto.getRandomValues(new Uint8Array(32));

  const allowCredentials: PublicKeyCredentialDescriptor[] = credentialIds.map((id) => ({
    type: 'public-key',
    id: base64UrlToBytes(id) as BufferSource,
  }));

  const prfInput =
    credentialIds.length === 1
      ? { eval: { first: saltsByCredentialId[credentialIds[0]] as BufferSource } }
      : {
          evalByCredential: Object.fromEntries(
            credentialIds.map((id) => [id, { first: saltsByCredentialId[id] as BufferSource }])
          ),
        };

  let assertion: PublicKeyCredential | null;
  try {
    assertion = (await navigator.credentials.get({
      publicKey: {
        challenge: challenge as BufferSource,
        allowCredentials,
        userVerification: 'required',
        extensions: { prf: prfInput } as AuthenticationExtensionsClientInputs,
      },
      signal: options.signal,
    })) as PublicKeyCredential | null;
  } catch (error) {
    if (isCancellation(error)) {
      throw new PrfCancelledError('Passkey prompt was dismissed');
    }
    throw error;
  }

  if (!assertion) {
    throw new PrfCancelledError('No credential was returned');
  }

  const extensions = assertion.getClientExtensionResults() as ExtensionResultsWithPrf;
  const first = extensions.prf?.results?.first;

  if (!first) {
    // The assertion itself succeeded, so the passkey is valid — the
    // authenticator or browser simply would not evaluate the PRF.
    throw new PrfUnsupportedError(
      'This passkey cannot unlock the vault — its authenticator does not support the PRF extension'
    );
  }

  return {
    credentialId: bytesToBase64Url(new Uint8Array(assertion.rawId)),
    prfOutput: new Uint8Array(first),
  };
}

/**
 * Evaluates the PRF for one specific credential, for enrolment.
 *
 * Enrolment needs the PRF output for the credential the user just chose, under
 * a freshly generated salt.
 */
export async function evaluatePrfForCredential(
  credentialId: string,
  salt: Uint8Array,
  options: { signal?: AbortSignal } = {}
): Promise<Uint8Array> {
  const { prfOutput } = await evaluatePrf({ [credentialId]: salt }, options);
  return prfOutput;
}

/**
 * Best-effort check that a credential can do PRF, without enrolling anything.
 *
 * There is no way to ask without a user gesture, so this performs a real
 * assertion. Callers should only run it in response to an explicit action.
 */
export async function probePrfSupport(credentialId: string): Promise<boolean> {
  try {
    await evaluatePrfForCredential(credentialId, crypto.getRandomValues(new Uint8Array(32)));
    return true;
  } catch (error) {
    if (error instanceof PrfUnsupportedError) {
      return false;
    }
    throw error;
  }
}
