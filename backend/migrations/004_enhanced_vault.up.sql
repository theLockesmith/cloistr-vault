-- Enhanced Vault Schema
-- Introduces granular entry management with folders, tags, multiple secrets, and attachments
-- Maintains zero-knowledge for sensitive data (secrets encrypted client-side)

-- ============================================
-- FOLDERS - Hierarchical organization
-- ============================================
CREATE TABLE IF NOT EXISTS vault_folders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES vault_folders(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    icon VARCHAR(50) DEFAULT '📁',
    color VARCHAR(7) DEFAULT '#6366f1',  -- hex color
    position INTEGER NOT NULL DEFAULT 0,

    -- For sharing (future)
    is_shared BOOLEAN DEFAULT FALSE,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Prevent duplicate folder names at same level for same user
    CONSTRAINT unique_folder_name_per_parent UNIQUE (user_id, parent_id, name)
);

CREATE INDEX idx_vault_folders_user_id ON vault_folders(user_id);
CREATE INDEX idx_vault_folders_parent_id ON vault_folders(parent_id);
CREATE INDEX idx_vault_folders_position ON vault_folders(user_id, parent_id, position);

-- ============================================
-- TAGS - Categorization system
-- ============================================
CREATE TABLE IF NOT EXISTS vault_tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    name VARCHAR(100) NOT NULL,
    color VARCHAR(7) DEFAULT '#6366f1',
    category VARCHAR(20) NOT NULL DEFAULT 'custom',  -- 'security', 'type', 'custom'
    is_system BOOLEAN DEFAULT FALSE,  -- auto-generated vs user-created
    usage_count INTEGER DEFAULT 0,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT vault_tags_category_check CHECK (category IN ('security', 'type', 'custom')),
    CONSTRAINT unique_tag_name_per_user UNIQUE (user_id, name)
);

CREATE INDEX idx_vault_tags_user_id ON vault_tags(user_id);
CREATE INDEX idx_vault_tags_category ON vault_tags(user_id, category);

-- ============================================
-- ENTRIES - Individual vault items
-- Metadata stored in cleartext, secrets encrypted client-side
-- ============================================
CREATE TABLE IF NOT EXISTS vault_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    folder_id UUID REFERENCES vault_folders(id) ON DELETE SET NULL,

    -- Basic info (cleartext for search/filtering)
    name VARCHAR(255) NOT NULL,
    entry_type VARCHAR(50) NOT NULL DEFAULT 'login',
    url VARCHAR(2048),
    notes TEXT,  -- markdown notes

    -- Organization
    is_favorite BOOLEAN DEFAULT FALSE,
    position INTEGER NOT NULL DEFAULT 0,

    -- Security analysis (computed client-side, stored for display)
    strength_score INTEGER DEFAULT 0,  -- 0-100
    has_weak_password BOOLEAN DEFAULT FALSE,
    has_reused_password BOOLEAN DEFAULT FALSE,
    has_breach BOOLEAN DEFAULT FALSE,
    last_breach_check TIMESTAMP WITH TIME ZONE,

    -- Usage tracking
    last_used TIMESTAMP WITH TIME ZONE,
    usage_count INTEGER DEFAULT 0,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT vault_entries_type_check CHECK (entry_type IN (
        'login', 'api_key', 'server', 'crypto_wallet', 'secure_note',
        'credit_card', 'identity', 'license', 'wifi', 'bank_account', 'custom'
    ))
);

CREATE INDEX idx_vault_entries_user_id ON vault_entries(user_id);
CREATE INDEX idx_vault_entries_folder_id ON vault_entries(folder_id);
CREATE INDEX idx_vault_entries_type ON vault_entries(user_id, entry_type);
CREATE INDEX idx_vault_entries_favorite ON vault_entries(user_id, is_favorite) WHERE is_favorite = TRUE;
CREATE INDEX idx_vault_entries_search ON vault_entries USING gin(to_tsvector('english', name || ' ' || COALESCE(url, '') || ' ' || COALESCE(notes, '')));
CREATE INDEX idx_vault_entries_last_used ON vault_entries(user_id, last_used DESC NULLS LAST);

-- ============================================
-- SECRETS - Encrypted values within entries
-- Values are encrypted client-side, server never sees plaintext
-- ============================================
CREATE TABLE IF NOT EXISTS vault_secrets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id UUID NOT NULL REFERENCES vault_entries(id) ON DELETE CASCADE,

    secret_type VARCHAR(50) NOT NULL DEFAULT 'password',
    name VARCHAR(255) NOT NULL,  -- "Login Password", "API Key", "SSH Private Key"

    -- Client-encrypted value (base64 encoded)
    encrypted_value TEXT NOT NULL,

    -- Optional metadata
    username VARCHAR(255),  -- Associated username for this secret
    expires_at TIMESTAMP WITH TIME ZONE,
    last_rotated TIMESTAMP WITH TIME ZONE,

    -- Security (computed client-side)
    strength_score INTEGER DEFAULT 0,

    position INTEGER NOT NULL DEFAULT 0,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT vault_secrets_type_check CHECK (secret_type IN (
        'password', 'username', 'api_key', 'app_password', 'recovery_code',
        'totp_secret', 'private_key', 'certificate', 'token', 'pin',
        'security_question', 'seed_phrase', 'custom'
    ))
);

CREATE INDEX idx_vault_secrets_entry_id ON vault_secrets(entry_id);
CREATE INDEX idx_vault_secrets_type ON vault_secrets(entry_id, secret_type);
CREATE INDEX idx_vault_secrets_expires ON vault_secrets(expires_at) WHERE expires_at IS NOT NULL;

-- ============================================
-- ENTRY-TAG RELATIONSHIPS - Many-to-many
-- ============================================
CREATE TABLE IF NOT EXISTS vault_entry_tags (
    entry_id UUID NOT NULL REFERENCES vault_entries(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES vault_tags(id) ON DELETE CASCADE,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    PRIMARY KEY (entry_id, tag_id)
);

CREATE INDEX idx_vault_entry_tags_tag_id ON vault_entry_tags(tag_id);

-- ============================================
-- ATTACHMENTS - Encrypted file storage
-- ============================================
CREATE TABLE IF NOT EXISTS vault_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id UUID NOT NULL REFERENCES vault_entries(id) ON DELETE CASCADE,

    name VARCHAR(255) NOT NULL,
    file_type VARCHAR(50) NOT NULL DEFAULT 'document',
    mime_type VARCHAR(255) NOT NULL,
    file_size INTEGER NOT NULL,  -- bytes

    -- Client-encrypted file data (base64 encoded)
    encrypted_data TEXT NOT NULL,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT vault_attachments_type_check CHECK (file_type IN (
        'image', 'document', 'key_file', 'certificate', 'backup', 'other'
    )),
    CONSTRAINT vault_attachments_size_check CHECK (file_size > 0 AND file_size <= 104857600)  -- max 100MB
);

CREATE INDEX idx_vault_attachments_entry_id ON vault_attachments(entry_id);

-- ============================================
-- ENTRY HISTORY - Audit trail for changes
-- ============================================
CREATE TABLE IF NOT EXISTS vault_entry_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entry_id UUID NOT NULL REFERENCES vault_entries(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    action VARCHAR(50) NOT NULL,
    changes JSONB,  -- snapshot of what changed

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    CONSTRAINT vault_entry_history_action_check CHECK (action IN (
        'created', 'updated', 'accessed', 'shared', 'secret_added',
        'secret_updated', 'secret_deleted', 'attachment_added', 'attachment_deleted'
    ))
);

CREATE INDEX idx_vault_entry_history_entry_id ON vault_entry_history(entry_id);
CREATE INDEX idx_vault_entry_history_user_id ON vault_entry_history(user_id);
CREATE INDEX idx_vault_entry_history_created ON vault_entry_history(created_at DESC);

-- ============================================
-- PASSWORD GENERATION HISTORY
-- ============================================
CREATE TABLE IF NOT EXISTS password_generation_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Generation settings used
    length INTEGER NOT NULL,
    include_uppercase BOOLEAN DEFAULT TRUE,
    include_lowercase BOOLEAN DEFAULT TRUE,
    include_numbers BOOLEAN DEFAULT TRUE,
    include_symbols BOOLEAN DEFAULT TRUE,

    -- Results
    strength_score INTEGER NOT NULL,
    entropy_bits DECIMAL(10,2) NOT NULL,

    -- Optional link to where it was used
    used_for_entry_id UUID REFERENCES vault_entries(id) ON DELETE SET NULL,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_password_history_user_id ON password_generation_history(user_id);
CREATE INDEX idx_password_history_created ON password_generation_history(created_at DESC);

-- ============================================
-- MIGRATION STATUS TRACKING
-- ============================================
ALTER TABLE users ADD COLUMN IF NOT EXISTS vault_migration_status VARCHAR(20) DEFAULT 'legacy';
-- 'legacy' = using old blob vault
-- 'migrating' = migration in progress
-- 'enhanced' = using new granular vault

COMMENT ON COLUMN users.vault_migration_status IS 'Tracks migration from blob vault to enhanced granular vault';

-- ============================================
-- TRIGGERS
-- ============================================

-- Update timestamps on modification
CREATE OR REPLACE FUNCTION update_vault_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_vault_folders_updated_at
    BEFORE UPDATE ON vault_folders
    FOR EACH ROW
    EXECUTE FUNCTION update_vault_updated_at();

CREATE TRIGGER update_vault_entries_updated_at
    BEFORE UPDATE ON vault_entries
    FOR EACH ROW
    EXECUTE FUNCTION update_vault_updated_at();

CREATE TRIGGER update_vault_secrets_updated_at
    BEFORE UPDATE ON vault_secrets
    FOR EACH ROW
    EXECUTE FUNCTION update_vault_updated_at();

-- Update tag usage count on entry-tag changes
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

CREATE TRIGGER update_tag_count_on_entry_tag
    AFTER INSERT OR DELETE ON vault_entry_tags
    FOR EACH ROW
    EXECUTE FUNCTION update_tag_usage_count();

-- ============================================
-- SYSTEM TAGS - Default security tags
-- ============================================
-- These will be created per-user on first vault access
-- Defined here for reference:
-- { name: "weak-password", color: "#ef4444", category: "security" }
-- { name: "reused-password", color: "#f59e0b", category: "security" }
-- { name: "2fa-enabled", color: "#10b981", category: "security" }
-- { name: "breach-detected", color: "#dc2626", category: "security" }
-- { name: "password-expired", color: "#f59e0b", category: "security" }
-- { name: "api-key", color: "#6366f1", category: "type" }

-- ============================================
-- COMMENTS
-- ============================================
COMMENT ON TABLE vault_folders IS 'Hierarchical folder organization for vault entries';
COMMENT ON TABLE vault_tags IS 'Tags for categorizing vault entries (security, type, custom)';
COMMENT ON TABLE vault_entries IS 'Individual vault items with metadata (secrets encrypted separately)';
COMMENT ON TABLE vault_secrets IS 'Client-encrypted secrets within entries - server never sees plaintext';
COMMENT ON TABLE vault_entry_tags IS 'Many-to-many relationship between entries and tags';
COMMENT ON TABLE vault_attachments IS 'Client-encrypted file attachments for entries';
COMMENT ON TABLE vault_entry_history IS 'Audit trail for entry modifications';
COMMENT ON TABLE password_generation_history IS 'History of generated passwords with settings used';
