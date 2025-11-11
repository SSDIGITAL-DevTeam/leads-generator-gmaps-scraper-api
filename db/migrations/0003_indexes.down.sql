-- Migration 0003 down: remove supporting indexes
DROP INDEX IF EXISTS idx_companies_city;
DROP INDEX IF EXISTS idx_companies_country;
DROP INDEX IF EXISTS idx_companies_type_business;
DROP INDEX IF EXISTS idx_companies_rating;
DROP INDEX IF EXISTS idx_companies_location;
