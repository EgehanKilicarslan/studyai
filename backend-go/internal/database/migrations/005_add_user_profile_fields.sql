-- +goose Up
-- Migration: Add username and full_name to users table
-- This migration adds username and full_name columns to support better user profiles

-- Step 1: Add username column (temporarily nullable)
ALTER TABLE users ADD COLUMN IF NOT EXISTS username VARCHAR(255);

-- Step 2: Add full_name column (temporarily nullable)
ALTER TABLE users ADD COLUMN IF NOT EXISTS full_name VARCHAR(255);

-- Step 3: Generate default usernames for existing users (email prefix)
UPDATE users 
SET username = CONCAT('user_', id)
WHERE username IS NULL;

-- Step 4: Set default full names for existing users (use email)
UPDATE users 
SET full_name = SPLIT_PART(email, '@', 1)
WHERE full_name IS NULL;

-- Step 5: Make columns NOT NULL
ALTER TABLE users ALTER COLUMN username SET NOT NULL;
ALTER TABLE users ALTER COLUMN full_name SET NOT NULL;

-- Step 6: Add unique index on username
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username) WHERE deleted_at IS NULL;

-- Add comment for documentation
COMMENT ON COLUMN users.username IS 'Unique username for user login and display';
COMMENT ON COLUMN users.full_name IS 'User full name for display purposes';

-- +goose Down
-- Rollback: Remove username and full_name columns
DROP INDEX IF EXISTS idx_users_username;
ALTER TABLE users DROP COLUMN username;
ALTER TABLE users DROP COLUMN full_name;
