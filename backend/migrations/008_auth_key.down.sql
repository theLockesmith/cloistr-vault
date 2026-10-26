-- Rolling back makes every v2 account unable to sign in: its verifier is over
-- the auth key, which the old code would compare against a password.
ALTER TABLE auth_methods DROP COLUMN IF EXISTS auth_version;
