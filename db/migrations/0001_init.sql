-- Migration 0001: initialize spatial-enabled companies table
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS companies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    place_id TEXT UNIQUE,
    company TEXT NOT NULL,
    phone TEXT,
    website TEXT,
    rating NUMERIC(2,1),
    reviews INT,
    type_business TEXT,
    address TEXT,
    city TEXT,
    country TEXT,
    location GEOGRAPHY(POINT, 4326),
    raw JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS unique_company_address
    ON companies (company, address)
    WHERE place_id IS NULL;
