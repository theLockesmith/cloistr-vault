-- Database initialization script for Docker
-- This runs when the PostgreSQL container starts for the first time

-- Create the database and user if they don't exist
-- Note: These commands might not be needed if using environment variables
-- but they're here for completeness

-- The database and user are already created by environment variables
-- but we can add any additional setup here if needed

-- Set timezone
SET timezone = 'UTC';

-- Enable necessary extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Log initialization
DO $$
BEGIN
    RAISE NOTICE 'Database initialization completed for Coldforge Vault';
END $$;