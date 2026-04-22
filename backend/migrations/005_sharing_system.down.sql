-- Rollback: 005_sharing_system

-- Drop indexes
DROP INDEX IF EXISTS idx_teams_owner;
DROP INDEX IF EXISTS idx_team_members_team;
DROP INDEX IF EXISTS idx_team_members_user;
DROP INDEX IF EXISTS idx_shared_folders_folder;
DROP INDEX IF EXISTS idx_shared_folders_team;
DROP INDEX IF EXISTS idx_shared_folders_user;
DROP INDEX IF EXISTS idx_shared_folder_keys_folder;
DROP INDEX IF EXISTS idx_shared_folder_keys_user;
DROP INDEX IF EXISTS idx_team_invitations_team;
DROP INDEX IF EXISTS idx_team_invitations_pubkey;

-- Remove sharing columns from vault_folders
ALTER TABLE vault_folders
DROP COLUMN IF EXISTS is_shared,
DROP COLUMN IF EXISTS folder_key_hash;

-- Drop tables in dependency order
DROP TABLE IF EXISTS team_invitations;
DROP TABLE IF EXISTS shared_folder_keys;
DROP TABLE IF EXISTS shared_folders;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
