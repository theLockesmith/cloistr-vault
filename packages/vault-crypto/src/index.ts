export {
  utf8ToBytes,
  bytesToUtf8,
  bytesToBase64,
  base64ToBytes,
  bytesToBase64Url,
  base64UrlToBytes,
  encodeJson,
  decodeJson,
} from './encoding';

export {
  ENVELOPE_VERSION,
  SCRYPT_PARAMS,
  isEnvelopeV2,
  serializeEnvelope,
  deserializeEnvelope,
  createEnvelope,
  unwrapWithPassword,
  unwrapWithPrf,
  prfSaltFor,
  enrolledPrfCredentialIds,
  decryptBody,
  encryptBody,
  addPrfWrap,
  removePrfWrap,
  newPrfSalt,
} from './envelope';
export type {
  ScryptParams,
  PasswordWrap,
  PrfWrap,
  KeyWrap,
  VaultEnvelopeV2,
} from './envelope';

export {
  unlockWithPassword,
  unlockWithPrf,
  createInitialEnvelope,
  saveVault,
  passkeyUnlockSalts,
  enrollPasskey,
  revokePasskey,
  serialize,
  wipe,
  VaultFormatError,
} from './vault';
export type { UnlockedVault } from './vault';

export { isLegacyVault, decryptLegacyVault } from './legacy';

export {
  randomBytes,
  randomInt,
  shuffle,
  randomChoice,
  generatePassword,
  generatePasswordFromOptions,
} from './random';
export type { PasswordOptions } from './random';

export { base32Decode, totp, totpSecondsRemaining } from './totp';

export {
  passwordStrength,
  findDuplicatePasswords,
} from './password-strength';
export type { StrengthResult } from './password-strength';
