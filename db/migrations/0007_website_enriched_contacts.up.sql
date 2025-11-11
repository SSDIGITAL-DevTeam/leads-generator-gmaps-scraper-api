-- Migration 0007: structured website enriched contacts
CREATE TABLE IF NOT EXISTS website_enriched_contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID UNIQUE NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    emails TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    phones TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    linkedin_url TEXT,
    facebook_url TEXT,
    instagram_url TEXT,
    youtube_url TEXT,
    tiktok_url TEXT,
    address TEXT,
    contact_form_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_website_enriched_contacts_updated_at
    ON website_enriched_contacts (updated_at DESC);
