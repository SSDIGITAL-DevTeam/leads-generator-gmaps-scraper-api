-- Migration 0005 down: remove scrape run metadata columns and indexes
DROP INDEX IF EXISTS idx_companies_scrape_run_id;
DROP INDEX IF EXISTS idx_companies_scraped_at;
ALTER TABLE companies
    DROP COLUMN IF EXISTS scrape_run_id,
    DROP COLUMN IF EXISTS scraped_at;
