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
} from '@cloistr/vault-crypto';
export type {
  ScryptParams,
  PasswordWrap,
  PrfWrap,
  KeyWrap,
  VaultEnvelopeV2,
} from '@cloistr/vault-crypto';
