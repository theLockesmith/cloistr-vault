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
  ENVELOPE_VERSION,
  SCRYPT_PARAMS,
  newPrfSalt,
  prfSaltFor,
  enrolledPrfCredentialIds,
} from '@cloistr/vault-crypto';
export type {
  UnlockedVault,
  VaultEnvelopeV2,
  KeyWrap,
  PasswordWrap,
  PrfWrap,
} from '@cloistr/vault-crypto';
