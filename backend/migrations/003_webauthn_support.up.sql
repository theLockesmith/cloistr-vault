-- Add WebAuthn/Passkey authentication support

-- Table for storing WebAuthn credentials (passkeys)
CREATE TABLE IF NOT EXISTS webauthn_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- WebAuthn spec fields
    credential_id BYTEA NOT NULL UNIQUE,
    public_key BYTEA NOT NULL,
    credential_name VARCHAR(255) DEFAULT 'Passkey',

    -- Authenticator metadata
    attestation_type VARCHAR(50) DEFAULT 'none',
    transports TEXT,
    sign_count INTEGER NOT NULL DEFAULT 0,
    aaguid BYTEA,

    -- Flags from authenticator data
    flags_user_present BOOLEAN DEFAULT FALSE,
    flags_user_verified BOOLEAN DEFAULT FALSE,
    flags_backup_eligible BOOLEAN DEFAULT FALSE,
    flags_backup_state BOOLEAN DEFAULT FALSE,

    -- Attestation data (optional, for verification)
    attestation_data JSONB,

    -- Lifecycle
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_used_at TIMESTAMP WITH TIME ZONE,

    CONSTRAINT fk_webauthn_user_id FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_webauthn_creds_user_id ON webauthn_credentials(user_id);
CREATE INDEX IF NOT EXISTS idx_webauthn_creds_credential_id ON webauthn_credentials(credential_id);
CREATE INDEX IF NOT EXISTS idx_webauthn_creds_last_used ON webauthn_credentials(user_id, last_used_at DESC);

-- Table for storing temporary WebAuthn session data (challenges)
CREATE TABLE IF NOT EXISTS webauthn_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,

    session_data BYTEA NOT NULL,
    ceremony_type VARCHAR(50) NOT NULL,
    challenge BYTEA NOT NULL,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,

    CONSTRAINT fk_webauthn_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_webauthn_sessions_user_id ON webauthn_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_webauthn_sessions_expires_at ON webauthn_sessions(expires_at);

-- Update the auth_methods type constraint to include webauthn
ALTER TABLE auth_methods DROP CONSTRAINT IF EXISTS auth_methods_type_check;
ALTER TABLE auth_methods ADD CONSTRAINT auth_methods_type_check
    CHECK (type IN ('email', 'nostr', 'lightning_address', 'nip05', 'webauthn'));

-- Add webauthn_credential_id column to auth_methods for linking
ALTER TABLE auth_methods
ADD COLUMN IF NOT EXISTS webauthn_credential_id UUID REFERENCES webauthn_credentials(id) ON DELETE SET NULL;

-- Update the validation constraint to include webauthn
ALTER TABLE auth_methods DROP CONSTRAINT IF EXISTS auth_methods_auth_validation;
ALTER TABLE auth_methods ADD CONSTRAINT auth_methods_auth_validation CHECK (
    (type = 'email' AND salt IS NOT NULL AND password_hash IS NOT NULL) OR
    (type = 'nostr' AND nostr_pubkey IS NOT NULL) OR
    (type = 'lightning_address' AND identifier IS NOT NULL) OR
    (type = 'nip05' AND identifier IS NOT NULL AND nostr_pubkey IS NOT NULL) OR
    (type = 'webauthn' AND webauthn_credential_id IS NOT NULL)
);

-- Comments
COMMENT ON TABLE webauthn_credentials IS 'Stores WebAuthn/Passkey credentials for passwordless authentication';
COMMENT ON COLUMN webauthn_credentials.credential_id IS 'Unique credential ID from authenticator';
COMMENT ON COLUMN webauthn_credentials.public_key IS 'DER-encoded public key for signature verification';
COMMENT ON COLUMN webauthn_credentials.sign_count IS 'Counter for detecting cloned authenticators';
COMMENT ON COLUMN webauthn_credentials.aaguid IS 'Authenticator AAGUID (16 bytes) for identifying authenticator type';
COMMENT ON COLUMN webauthn_credentials.flags_backup_eligible IS 'Whether credential can be synced across devices';
COMMENT ON TABLE webauthn_sessions IS 'Temporary storage for WebAuthn challenge sessions';
