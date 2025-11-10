-- Migration 0003: add supporting indexes for query performance
CREATE INDEX IF NOT EXISTS idx_companies_city ON companies USING BTREE (city);
CREATE INDEX IF NOT EXISTS idx_companies_country ON companies USING BTREE (country);
CREATE INDEX IF NOT EXISTS idx_companies_type_business ON companies USING BTREE (type_business);
CREATE INDEX IF NOT EXISTS idx_companies_rating ON companies USING BTREE (rating);
CREATE INDEX IF NOT EXISTS idx_companies_location ON companies USING GIST (location);
