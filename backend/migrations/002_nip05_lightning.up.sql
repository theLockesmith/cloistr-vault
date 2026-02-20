-- Add NIP-05 and Lightning authentication support

-- Add new columns to auth_methods
ALTER TABLE auth_methods
ADD COLUMN IF NOT EXISTS nip05_address VARCHAR(255),
ADD COLUMN IF NOT EXISTS nip05_verified_at TIMESTAMP WITH TIME ZONE,
ADD COLUMN IF NOT EXISTS nip05_relays TEXT;

-- Update the type constraint to include lightning_address
ALTER TABLE auth_methods DROP CONSTRAINT IF EXISTS auth_methods_type_check;
ALTER TABLE auth_methods ADD CONSTRAINT auth_methods_type_check
    CHECK (type IN ('email', 'nostr', 'lightning_address', 'nip05'));

-- Update the validation constraint
ALTER TABLE auth_methods DROP CONSTRAINT IF EXISTS auth_methods_email_check;
ALTER TABLE auth_methods ADD CONSTRAINT auth_methods_auth_validation CHECK (
    (type = 'email' AND salt IS NOT NULL AND password_hash IS NOT NULL) OR
    (type = 'nostr' AND nostr_pubkey IS NOT NULL) OR
    (type = 'lightning_address' AND identifier IS NOT NULL) OR
    (type = 'nip05' AND identifier IS NOT NULL AND nostr_pubkey IS NOT NULL)
);

-- Index for NIP-05 lookups
CREATE INDEX IF NOT EXISTS idx_auth_methods_nip05_address
    ON auth_methods(nip05_address)
    WHERE nip05_address IS NOT NULL;

-- Comments
COMMENT ON COLUMN auth_methods.nip05_address IS 'Verified NIP-05 address (e.g., alice@cloistr.xyz)';
COMMENT ON COLUMN auth_methods.nip05_verified_at IS 'When the NIP-05 address was last verified';
COMMENT ON COLUMN auth_methods.nip05_relays IS 'Comma-separated list of recommended relays from NIP-05';
