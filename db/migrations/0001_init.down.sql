-- Migration 0001 down: drop companies catalog and supporting extensions
DROP INDEX IF EXISTS unique_company_address;
DROP TABLE IF EXISTS companies;
DROP EXTENSION IF EXISTS postgis;
DROP EXTENSION IF EXISTS pgcrypto;
