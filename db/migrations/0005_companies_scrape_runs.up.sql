-- Migration 0005: track scrape run metadata on companies
ALTER TABLE companies
    ADD COLUMN IF NOT EXISTS scrape_run_id UUID,
    ADD COLUMN IF NOT EXISTS scraped_at TIMESTAMPTZ;

UPDATE companies
SET scraped_at = COALESCE(scraped_at, updated_at)
WHERE scraped_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_companies_scrape_run_id ON companies (scrape_run_id);
CREATE INDEX IF NOT EXISTS idx_companies_scraped_at ON companies (scraped_at);
