-- Email accounts used to sign in with the master password itself, so the
-- server saw the one secret that decrypts the vault. They now sign in with an
-- auth key the client derives from it (scrypt over an email-derived salt).
--
-- auth_version says what password_hash verifies:
--   1 = the master password (every account created before this migration)
--   2 = the client-derived auth key; the password is never accepted again
-- A v1 account becomes v2 on its first sign-in from a client that sends both.
ALTER TABLE auth_methods ADD COLUMN IF NOT EXISTS auth_version SMALLINT NOT NULL DEFAULT 1;
