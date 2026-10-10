-- Migration: 007_rate_limits
-- Description: Per-client request counters, shared by every replica so a
-- client's limit does not double with the replica count. One row per
-- (bucket, address). expires_at is set when a window opens and is never moved
-- by later hits in that window (fixed window): a window that slid forward on
-- every hit would never close for a client that keeps retrying.
--
-- UNLOGGED: the counters are worthless after a crash or failover, and skipping
-- WAL keeps the per-request write cheap. Losing them only resets windows.

CREATE UNLOGGED TABLE IF NOT EXISTS rate_limits (
    key TEXT PRIMARY KEY,
    count INTEGER NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_rate_limits_expires_at ON rate_limits(expires_at);
