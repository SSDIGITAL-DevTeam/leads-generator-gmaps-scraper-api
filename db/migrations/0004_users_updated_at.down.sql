-- Migration 0004 down: revert users updated_at trigger and column
DROP TRIGGER IF EXISTS set_timestamp ON users;
DROP FUNCTION IF EXISTS trigger_set_timestamp();
ALTER TABLE users
    DROP COLUMN IF EXISTS updated_at;
