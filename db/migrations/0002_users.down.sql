-- Migration 0002 down: drop users table and role enum
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS user_role;
