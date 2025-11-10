-- Migration 0006: store enrichment payloads per company
CREATE TABLE IF NOT EXISTS company_enrichments (
    company_id UUID PRIMARY KEY REFERENCES companies(id) ON DELETE CASCADE,
    emails TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    phones TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    socials JSONB NOT NULL DEFAULT '{}'::jsonb,
    address TEXT,
    contact_form_url TEXT,
    about_summary TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_company_enrichments_updated_at
    ON company_enrichments (updated_at DESC);
