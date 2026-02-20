-- Rollback WebAuthn/Passkey authentication support

-- Remove webauthn_credential_id from auth_methods
ALTER TABLE auth_methods DROP COLUMN IF EXISTS webauthn_credential_id;

-- Restore the previous type constraint (without webauthn)
ALTER TABLE auth_methods DROP CONSTRAINT IF EXISTS auth_methods_type_check;
ALTER TABLE auth_methods ADD CONSTRAINT auth_methods_type_check
    CHECK (type IN ('email', 'nostr', 'lightning_address', 'nip05'));

-- Restore the previous validation constraint (without webauthn)
ALTER TABLE auth_methods DROP CONSTRAINT IF EXISTS auth_methods_auth_validation;
ALTER TABLE auth_methods ADD CONSTRAINT auth_methods_auth_validation CHECK (
    (type = 'email' AND salt IS NOT NULL AND password_hash IS NOT NULL) OR
    (type = 'nostr' AND nostr_pubkey IS NOT NULL) OR
    (type = 'lightning_address' AND identifier IS NOT NULL) OR
    (type = 'nip05' AND identifier IS NOT NULL AND nostr_pubkey IS NOT NULL)
);

-- Drop indexes
DROP INDEX IF EXISTS idx_webauthn_sessions_expires_at;
DROP INDEX IF EXISTS idx_webauthn_sessions_user_id;
DROP INDEX IF EXISTS idx_webauthn_creds_last_used;
DROP INDEX IF EXISTS idx_webauthn_creds_credential_id;
DROP INDEX IF EXISTS idx_webauthn_creds_user_id;

-- Drop tables
DROP TABLE IF EXISTS webauthn_sessions;
DROP TABLE IF EXISTS webauthn_credentials;
