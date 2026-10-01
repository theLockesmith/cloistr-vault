-- Migration: 006_shared_challenges
-- Description: Move login challenges out of per-pod memory so a challenge issued
-- by one replica can be redeemed on another. Single use is enforced by
-- DELETE ... (only the request whose delete removes the row is admitted).

CREATE TABLE IF NOT EXISTS auth_challenges (
    id TEXT PRIMARY KEY,
    value TEXT NOT NULL UNIQUE,
    user_id UUID,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auth_challenges_expires_at ON auth_challenges(expires_at);
