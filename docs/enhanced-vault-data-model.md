-- Enhanced Vault Schema for Professional Password Manager Features
-- Adds folders, tags, multiple secrets, and enhanced organization

-- Vault folders for organization
CREATE TABLE vault_folders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    parent_id UUID REFERENCES vault_folders(id) ON DELETE CASCADE,
    icon VARCHAR(50) DEFAULT '📁',
    color VARCHAR(7) DEFAULT '#2563eb', -- hex color
    position INTEGER NOT NULL DEFAULT 0,
    is_shared BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Prevent circular references
    CONSTRAINT no_self_reference CHECK (id != parent_id),
    UNIQUE(user_id, name, parent_id) -- unique names within parent
);

-- Tags for categorization
CREATE TABLE vault_tags (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    color VARCHAR(7) DEFAULT '#64748b',
    category VARCHAR(50) DEFAULT 'custom', -- 'security', 'type', 'custom'
    is_system BOOLEAN DEFAULT FALSE, -- system-generated vs user tags
    usage_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    UNIQUE(user_id, name)
);

-- Enhanced vault entries
CREATE TABLE vault_entries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    folder_id UUID REFERENCES vault_folders(id) ON DELETE SET NULL,

    -- Basic info
    name VARCHAR(255) NOT NULL,
    entry_type VARCHAR(50) NOT NULL DEFAULT 'login', -- 'login', 'note', 'card', etc.
    notes TEXT, -- markdown notes
    url VARCHAR(1000),

    -- Organization
    is_favorite BOOLEAN DEFAULT FALSE,
    position INTEGER NOT NULL DEFAULT 0,

    -- Usage tracking
    last_used TIMESTAMP WITH TIME ZONE,
    usage_count INTEGER DEFAULT 0,

    -- Security metadata
    strength_score INTEGER, -- 0-100 password strength
    has_breach BOOLEAN DEFAULT FALSE,
    last_breach_check TIMESTAMP WITH TIME ZONE,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Constraints
    CONSTRAINT valid_entry_type CHECK (entry_type IN (
        'login', 'secure_note', 'credit_card', 'identity', 'api_key',
        'server', 'wifi', 'license', 'bank_account', 'crypto_wallet', 'custom'
    ))
);

-- Multiple secrets per entry
CREATE TABLE vault_secrets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    entry_id UUID NOT NULL REFERENCES vault_entries(id) ON DELETE CASCADE,

    -- Secret identification
    secret_type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL, -- "Login Password", "API Key", etc.
    encrypted_value BYTEA NOT NULL, -- client-encrypted secret value

    -- Security metadata
    expires_at TIMESTAMP WITH TIME ZONE,
    last_rotated TIMESTAMP WITH TIME ZONE,
    strength_score INTEGER,
    breach_status VARCHAR(20) DEFAULT 'safe', -- 'safe', 'warning', 'compromised'

    -- Usage tracking
    last_accessed TIMESTAMP WITH TIME ZONE,
    access_count INTEGER DEFAULT 0,

    -- Validation
    is_valid BOOLEAN DEFAULT TRUE,
    validated_at TIMESTAMP WITH TIME ZONE,
    validation_error TEXT,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT valid_secret_type CHECK (secret_type IN (
        'password', 'username', 'api_key', 'app_password', 'recovery_code',
        'totp_secret', 'private_key', 'certificate', 'token', 'pin',
        'security_question', 'custom'
    ))
);

-- Entry-tag relationships (many-to-many)
CREATE TABLE vault_entry_tags (
    entry_id UUID NOT NULL REFERENCES vault_entries(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES vault_tags(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    PRIMARY KEY (entry_id, tag_id)
);

-- Attachments (encrypted files)
CREATE TABLE vault_attachments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    entry_id UUID NOT NULL REFERENCES vault_entries(id) ON DELETE CASCADE,

    -- File metadata
    name VARCHAR(255) NOT NULL,
    file_type VARCHAR(50) NOT NULL, -- 'image', 'document', 'key_file', 'certificate'
    mime_type VARCHAR(100),
    file_size INTEGER NOT NULL,

    -- Encrypted content
    encrypted_data BYTEA NOT NULL,
    encryption_nonce BYTEA,

    -- Metadata
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT valid_file_type CHECK (file_type IN (
        'image', 'document', 'key_file', 'certificate', 'backup', 'other'
    ))
);

-- Entry access history (for security monitoring)
CREATE TABLE vault_entry_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    entry_id UUID NOT NULL REFERENCES vault_entries(id) ON DELETE CASCADE,

    -- Change tracking
    action VARCHAR(50) NOT NULL, -- 'created', 'updated', 'accessed', 'shared'
    field_changed VARCHAR(100), -- which field was modified
    old_value_hash BYTEA, -- hash of old value for integrity

    -- Context
    ip_address INET,
    user_agent TEXT,
    device_info JSONB,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT valid_action CHECK (action IN (
        'created', 'updated', 'accessed', 'shared', 'exported', 'deleted'
    ))
);

-- Password generation history
CREATE TABLE password_generation_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Generation settings
    settings JSONB NOT NULL, -- generation parameters
    password_hash BYTEA NOT NULL, -- hash for duplicate detection
    strength_score INTEGER,

    -- Usage
    used_for_entry_id UUID REFERENCES vault_entries(id) ON DELETE SET NULL,
    is_used BOOLEAN DEFAULT FALSE,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX idx_vault_folders_user_id ON vault_folders(user_id);
CREATE INDEX idx_vault_folders_parent_id ON vault_folders(parent_id);
CREATE INDEX idx_vault_tags_user_id ON vault_tags(user_id);
CREATE INDEX idx_vault_entries_user_id ON vault_entries(user_id);
CREATE INDEX idx_vault_entries_folder_id ON vault_entries(folder_id);
CREATE INDEX idx_vault_entries_type ON vault_entries(entry_type);
CREATE INDEX idx_vault_entries_favorite ON vault_entries(is_favorite) WHERE is_favorite = TRUE;
CREATE INDEX idx_vault_secrets_entry_id ON vault_secrets(entry_id);
CREATE INDEX idx_vault_secrets_type ON vault_secrets(secret_type);
CREATE INDEX idx_vault_entry_tags_entry ON vault_entry_tags(entry_id);
CREATE INDEX idx_vault_entry_tags_tag ON vault_entry_tags(tag_id);
CREATE INDEX idx_vault_attachments_entry_id ON vault_attachments(entry_id);
CREATE INDEX idx_vault_entry_history_entry_id ON vault_entry_history(entry_id);
CREATE INDEX idx_vault_entry_history_created_at ON vault_entry_history(created_at);

-- Full-text search indexes
CREATE INDEX idx_vault_entries_search ON vault_entries USING gin(to_tsvector('english', name || ' ' || COALESCE(notes, '')));

-- Add triggers for updated_at
CREATE TRIGGER update_vault_folders_updated_at
    BEFORE UPDATE ON vault_folders
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_vault_entries_updated_at
    BEFORE UPDATE ON vault_entries
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_vault_secrets_updated_at
    BEFORE UPDATE ON vault_secrets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Function to update tag usage counts
CREATE OR REPLACE FUNCTION update_tag_usage_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE vault_tags SET usage_count = usage_count + 1 WHERE id = NEW.tag_id;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE vault_tags SET usage_count = usage_count - 1 WHERE id = OLD.tag_id;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- Trigger for tag usage counting
CREATE TRIGGER vault_entry_tags_usage_trigger
    AFTER INSERT OR DELETE ON vault_entry_tags
    FOR EACH ROW EXECUTE FUNCTION update_tag_usage_count();

-- Comments for documentation
COMMENT ON TABLE vault_folders IS 'Hierarchical folder structure for organizing vault entries';
COMMENT ON TABLE vault_tags IS 'Tags for categorizing and filtering vault entries';
COMMENT ON TABLE vault_entries IS 'Enhanced vault entries with organization and metadata';
COMMENT ON TABLE vault_secrets IS 'Multiple secrets per entry (passwords, API keys, etc.)';
COMMENT ON TABLE vault_entry_tags IS 'Many-to-many relationship between entries and tags';
COMMENT ON TABLE vault_attachments IS 'Encrypted file attachments for vault entries';
COMMENT ON TABLE vault_entry_history IS 'Audit trail for entry modifications and access';
COMMENT ON TABLE password_generation_history IS 'History of generated passwords for security analysis';

-- Insert default system tags
INSERT INTO vault_tags (user_id, name, color, category, is_system)
SELECT id, 'weak-password', '#ef4444', 'security', TRUE FROM users
UNION ALL
SELECT id, 'reused-password', '#f59e0b', 'security', TRUE FROM users
UNION ALL
SELECT id, '2fa-enabled', '#10b981', 'security', TRUE FROM users
UNION ALL
SELECT id, 'breach-detected', '#dc2626', 'security', TRUE FROM users
UNION ALL
SELECT id, 'password-expired', '#f59e0b', 'security', TRUE FROM users
UNION ALL
SELECT id, 'shared-account', '#6366f1', 'type', TRUE FROM users;