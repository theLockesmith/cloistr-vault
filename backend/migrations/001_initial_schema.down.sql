-- Drop triggers
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TRIGGER IF EXISTS update_auth_methods_updated_at ON auth_methods;

-- Drop functions
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP FUNCTION IF EXISTS cleanup_expired_sessions();

-- Drop indexes (they'll be dropped automatically with tables, but explicit for clarity)
DROP INDEX IF EXISTS idx_auth_methods_user_id;
DROP INDEX IF EXISTS idx_auth_methods_type_identifier;
DROP INDEX IF EXISTS idx_auth_methods_nostr_pubkey;
DROP INDEX IF EXISTS idx_vaults_user_id;
DROP INDEX IF EXISTS idx_recovery_codes_user_id;
DROP INDEX IF EXISTS idx_recovery_codes_used;
DROP INDEX IF EXISTS idx_sessions_user_id;
DROP INDEX IF EXISTS idx_sessions_token;
DROP INDEX IF EXISTS idx_sessions_expires_at;
DROP INDEX IF EXISTS idx_audit_logs_user_id;
DROP INDEX IF EXISTS idx_audit_logs_created_at;
DROP INDEX IF EXISTS idx_audit_logs_action;

-- Drop tables (order matters due to foreign keys)
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS recovery_codes;
DROP TABLE IF EXISTS vaults;
DROP TABLE IF EXISTS auth_methods;
DROP TABLE IF EXISTS users;

-- Drop extensions (commented out to avoid conflicts with other applications)
-- DROP EXTENSION IF EXISTS "uuid-ossp";