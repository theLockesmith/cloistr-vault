-- Revert NIP-05 and Lightning authentication support

-- Drop index
DROP INDEX IF EXISTS idx_auth_methods_nip05_address;

-- Remove new columns
ALTER TABLE auth_methods
DROP COLUMN IF EXISTS nip05_address,
DROP COLUMN IF EXISTS nip05_verified_at,
DROP COLUMN IF EXISTS nip05_relays;

-- Restore original constraints
ALTER TABLE auth_methods DROP CONSTRAINT IF EXISTS auth_methods_type_check;
ALTER TABLE auth_methods ADD CONSTRAINT auth_methods_type_check
    CHECK (type IN ('email', 'nostr'));

ALTER TABLE auth_methods DROP CONSTRAINT IF EXISTS auth_methods_auth_validation;
ALTER TABLE auth_methods ADD CONSTRAINT auth_methods_email_check CHECK (
    (type = 'email' AND salt IS NOT NULL AND password_hash IS NOT NULL) OR
    (type = 'nostr' AND nostr_pubkey IS NOT NULL)
);
