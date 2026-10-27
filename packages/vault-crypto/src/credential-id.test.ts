/**
 * Credential IDs reach the vault padded: the server encodes them with Go's
 * base64.URLEncoding ("...=="), and enrolment stored them as given. Browsers
 * use unpadded base64url. Two failures followed (found by the WV4 live sweep,
 * VAULT-6): Chrome rejects padded keys in PRF `evalByCredential`, so unlock
 * failed whenever two or more passkeys were enrolled, and the assertion's
 * unpadded rawId never matched a padded wrap, so unlock failed for any
 * credential whose ID length is not a multiple of three bytes.
 */
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  createInitialEnvelope,
  unlockWithPassword,
  enrollPasskey,
  unlockWithPrf,
  passkeyUnlockSalts,
  revokePasskey,
  serialize,
  bytesToBase64,
} from './index';
import { evaluatePrf } from './prf';

const PW = 'correct horse battery staple';
const prfOutput = new Uint8Array(32).fill(7);
const salt = new Uint8Array(32).fill(3);

/** 16 bytes, as Go's base64.URLEncoding writes it: ends in "==". */
function paddedId(fill: number): string {
  return bytesToBase64(new Uint8Array(16).fill(fill)).replace(/\+/g, '-').replace(/\//g, '_');
}
const unpadded = (id: string) => id.replace(/=+$/, '');

async function vaultWithPasskey(credentialId: string) {
  const blob = await createInitialEnvelope({ entries: ['x'] }, PW);
  const session = await unlockWithPassword<{ entries: string[] }>(blob, PW, { entries: [] });
  return enrollPasskey(session, credentialId, salt, prfOutput, 'key');
}

describe('passkey credential IDs', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('unlocks with the browser-reported (unpadded) ID of a passkey enrolled with a padded one', async () => {
    const id = paddedId(1);
    expect(id.endsWith('==')).toBe(true);
    const blob = serialize(await vaultWithPasskey(id));

    const opened = await unlockWithPrf<{ entries: string[] }>(blob, unpadded(id), prfOutput);

    expect(opened.data.entries).toEqual(['x']);
  });

  it('lists enrolled IDs unpadded, and revokes by either form', async () => {
    const id = paddedId(2);
    const session = await vaultWithPasskey(id);

    expect(Object.keys(passkeyUnlockSalts(serialize(session)))).toEqual([unpadded(id)]);
    expect(Object.keys(passkeyUnlockSalts(serialize(revokePasskey(session, id))))).toEqual([]);
  });

  it('sends unpadded keys in evalByCredential', async () => {
    const get = vi.fn(async () => ({
      rawId: new Uint8Array(16).fill(1).buffer,
      getClientExtensionResults: () => ({ prf: { results: { first: prfOutput.buffer } } }),
    }));
    vi.stubGlobal('window', { PublicKeyCredential: function PublicKeyCredential() {} });
    vi.stubGlobal('navigator', { credentials: { get } });

    const result = await evaluatePrf({ [paddedId(1)]: salt, [paddedId(2)]: salt });

    const keys = Object.keys((get.mock.calls[0] as any)[0].publicKey.extensions.prf.evalByCredential);
    expect(keys).toEqual([unpadded(paddedId(1)), unpadded(paddedId(2))]);
    expect(result.credentialId).toBe(unpadded(paddedId(1)));
  });
});
