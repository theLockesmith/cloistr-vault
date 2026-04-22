-- Rollback enhanced vault schema
-- WARNING: This will delete all enhanced vault data!

-- Drop triggers first
DROP TRIGGER IF EXISTS update_tag_count_on_entry_tag ON vault_entry_tags;
DROP TRIGGER IF EXISTS update_vault_secrets_updated_at ON vault_secrets;
DROP TRIGGER IF EXISTS update_vault_entries_updated_at ON vault_entries;
DROP TRIGGER IF EXISTS update_vault_folders_updated_at ON vault_folders;

-- Drop functions
DROP FUNCTION IF EXISTS update_tag_usage_count();
DROP FUNCTION IF EXISTS update_vault_updated_at();

-- Drop tables in dependency order
DROP TABLE IF EXISTS password_generation_history;
DROP TABLE IF EXISTS vault_entry_history;
DROP TABLE IF EXISTS vault_attachments;
DROP TABLE IF EXISTS vault_entry_tags;
DROP TABLE IF EXISTS vault_secrets;
DROP TABLE IF EXISTS vault_entries;
DROP TABLE IF EXISTS vault_tags;
DROP TABLE IF EXISTS vault_folders;

-- Remove migration status column
ALTER TABLE users DROP COLUMN IF EXISTS vault_migration_status;
