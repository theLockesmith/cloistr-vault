-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Auth methods table (supports multiple auth methods per user)
CREATE TABLE auth_methods (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL, -- 'email', 'nostr'
    identifier VARCHAR(255) NOT NULL, -- email or nostr pubkey
    salt BYTEA, -- For password-based auth
    password_hash BYTEA, -- For password-based auth
    nostr_pubkey VARCHAR(64), -- For Nostr auth (hex encoded)
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT auth_methods_type_check CHECK (type IN ('email', 'nostr')),
    CONSTRAINT auth_methods_email_check CHECK (
        (type = 'email' AND salt IS NOT NULL AND password_hash IS NOT NULL) OR
        (type = 'nostr' AND nostr_pubkey IS NOT NULL)
    ),
    UNIQUE(type, identifier)
);

-- Vaults table (encrypted user data)
CREATE TABLE vaults (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    encrypted_data BYTEA NOT NULL, -- Client-encrypted vault data
    encryption_salt BYTEA, -- Salt used for client-side encryption (if any)
    encryption_nonce BYTEA, -- Nonce used for AES-GCM (if any)
    version INTEGER NOT NULL DEFAULT 1, -- For sync conflict resolution
    last_modified TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- One vault per user
    UNIQUE(user_id)
);

-- Recovery codes table (for account recovery)
CREATE TABLE recovery_codes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash BYTEA NOT NULL, -- Hashed recovery code
    salt BYTEA NOT NULL, -- Salt for recovery code hash
    used BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    used_at TIMESTAMP WITH TIME ZONE
);

-- Sessions table (active user sessions)
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Audit log table (optional, for security monitoring)
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL, -- 'login', 'register', 'vault_update', etc.
    details JSONB,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_auth_methods_user_id ON auth_methods(user_id);
CREATE INDEX idx_auth_methods_type_identifier ON auth_methods(type, identifier);
CREATE INDEX idx_auth_methods_nostr_pubkey ON auth_methods(nostr_pubkey) WHERE nostr_pubkey IS NOT NULL;
CREATE INDEX idx_vaults_user_id ON vaults(user_id);
CREATE INDEX idx_recovery_codes_user_id ON recovery_codes(user_id);
CREATE INDEX idx_recovery_codes_used ON recovery_codes(used) WHERE NOT used;
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_token ON sessions(token);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);

-- Functions for updated_at timestamps
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers for updated_at
CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_auth_methods_updated_at 
    BEFORE UPDATE ON auth_methods 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Function to clean up expired sessions
CREATE OR REPLACE FUNCTION cleanup_expired_sessions()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    DELETE FROM sessions WHERE expires_at < NOW();
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$ LANGUAGE 'plpgsql';

-- Comments for documentation
COMMENT ON TABLE users IS 'User accounts in the system';
COMMENT ON TABLE auth_methods IS 'Authentication methods (email/password, Nostr keypairs)';
COMMENT ON TABLE vaults IS 'Encrypted user password vaults (zero-knowledge)';
COMMENT ON TABLE recovery_codes IS 'Emergency recovery codes for account recovery';
COMMENT ON TABLE sessions IS 'Active user sessions';
COMMENT ON TABLE audit_logs IS 'Security audit log for monitoring';

COMMENT ON COLUMN vaults.encrypted_data IS 'Client-encrypted vault data - server cannot decrypt';
COMMENT ON COLUMN auth_methods.type IS 'Authentication method: email or nostr';
COMMENT ON COLUMN auth_methods.identifier IS 'Email address or Nostr public key';
COMMENT ON COLUMN auth_methods.nostr_pubkey IS 'Nostr public key in hex format (64 chars)';
COMMENT ON COLUMN recovery_codes.code_hash IS 'Hash of recovery code - not stored in plaintext';
COMMENT ON COLUMN sessions.token IS 'Session token (JWT in production)';
COMMENT ON COLUMN audit_logs.details IS 'Additional context data as JSON';