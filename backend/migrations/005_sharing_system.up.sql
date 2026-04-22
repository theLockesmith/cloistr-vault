-- Migration: 005_sharing_system
-- Description: Add teams, shared folders, and folder sharing with Nostr key exchange

-- Teams/Organizations table
CREATE TABLE IF NOT EXISTS teams (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Team members with roles
CREATE TABLE IF NOT EXISTS team_members (
    id UUID PRIMARY KEY,
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(team_id, user_id)
);

-- Shared folders - links a folder to a team with permission level
CREATE TABLE IF NOT EXISTS shared_folders (
    id UUID PRIMARY KEY,
    folder_id UUID NOT NULL REFERENCES vault_folders(id) ON DELETE CASCADE,
    team_id UUID REFERENCES teams(id) ON DELETE CASCADE,
    shared_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    shared_with UUID REFERENCES users(id) ON DELETE CASCADE, -- Direct user share (if not team)
    permission_level VARCHAR(20) NOT NULL DEFAULT 'view' CHECK (permission_level IN ('view', 'edit', 'admin')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE, -- Optional expiration
    -- Either team_id or shared_with must be set
    CHECK (team_id IS NOT NULL OR shared_with IS NOT NULL)
);

-- Folder encryption keys shared with users (encrypted with their Nostr pubkey)
-- Each folder has a symmetric folder_key; this stores that key encrypted for each recipient
CREATE TABLE IF NOT EXISTS shared_folder_keys (
    id UUID PRIMARY KEY,
    folder_id UUID NOT NULL REFERENCES vault_folders(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    encrypted_folder_key TEXT NOT NULL, -- Folder key encrypted with user's Nostr pubkey
    key_version INT NOT NULL DEFAULT 1, -- For key rotation
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(folder_id, user_id, key_version)
);

-- Add sharing metadata to folders
ALTER TABLE vault_folders
ADD COLUMN IF NOT EXISTS is_shared BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS folder_key_hash VARCHAR(64); -- Hash of folder key for verification

-- Team invitations
CREATE TABLE IF NOT EXISTS team_invitations (
    id UUID PRIMARY KEY,
    team_id UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    invited_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invited_email VARCHAR(255), -- For email invites
    invited_pubkey VARCHAR(128), -- For Nostr invites
    role VARCHAR(20) NOT NULL DEFAULT 'member',
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'declined', 'expired')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() + INTERVAL '7 days',
    accepted_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_teams_owner ON teams(owner_id);
CREATE INDEX IF NOT EXISTS idx_team_members_team ON team_members(team_id);
CREATE INDEX IF NOT EXISTS idx_team_members_user ON team_members(user_id);
CREATE INDEX IF NOT EXISTS idx_shared_folders_folder ON shared_folders(folder_id);
CREATE INDEX IF NOT EXISTS idx_shared_folders_team ON shared_folders(team_id);
CREATE INDEX IF NOT EXISTS idx_shared_folders_user ON shared_folders(shared_with);
CREATE INDEX IF NOT EXISTS idx_shared_folder_keys_folder ON shared_folder_keys(folder_id);
CREATE INDEX IF NOT EXISTS idx_shared_folder_keys_user ON shared_folder_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_team_invitations_team ON team_invitations(team_id);
CREATE INDEX IF NOT EXISTS idx_team_invitations_pubkey ON team_invitations(invited_pubkey);
