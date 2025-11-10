"""Database helpers for the worker."""

import logging
from contextlib import contextmanager
from typing import Any, Dict, Optional

import psycopg2
from psycopg2 import extras, pool

from src.core.config import get_settings

logger = logging.getLogger(__name__)

_connection_pool: Optional[pool.SimpleConnectionPool] = None


def init_pool(minconn: int = 1, maxconn: int = 5) -> pool.SimpleConnectionPool:
    """Initialise and return the shared connection pool."""
    global _connection_pool
    if _connection_pool is None:
        settings = get_settings()
        if not settings.database_url:
            raise RuntimeError("DATABASE_URL is required for database connections")
        _connection_pool = pool.SimpleConnectionPool(
            minconn,
            maxconn,
            dsn=settings.database_url,
            connect_timeout=10,
        )
        logger.info("Database connection pool initialised")
    return _connection_pool


@contextmanager
def get_connection():
    """Context manager yielding a pooled connection."""
    pg_pool = init_pool()
    conn = pg_pool.getconn()
    try:
        yield conn
    finally:
        pg_pool.putconn(conn)


def _prepare_params(row: Dict[str, Any]) -> Dict[str, Any]:
    scrape_run_id = row.get("scrape_run_id")
    if scrape_run_id is not None:
        scrape_run_id = str(scrape_run_id)

    return {
        "place_id": row.get("place_id"),
        "company": row.get("company"),
        "phone": row.get("phone"),
        "website": row.get("website"),
        "rating": row.get("rating"),
        "reviews": row.get("reviews"),
        "type_business": row.get("type_business"),
        "address": row.get("address"),
        "city": row.get("city"),
        "country": row.get("country"),
        "lng": row.get("lng"),
        "lat": row.get("lat"),
        "raw": extras.Json(row.get("raw") or {}),
        "scrape_run_id": scrape_run_id,
        "scraped_at": row.get("scraped_at"),
    }


_UPSERT_WITH_PLACE_ID = """
INSERT INTO companies (
    place_id,
    company,
    phone,
    website,
    rating,
    reviews,
    type_business,
    address,
    city,
    country,
    location,
    raw,
    scrape_run_id,
    scraped_at,
    updated_at
) VALUES (
    %(place_id)s,
    %(company)s,
    %(phone)s,
    %(website)s,
    %(rating)s,
    %(reviews)s,
    %(type_business)s,
    %(address)s,
    %(city)s,
    %(country)s,
    CASE WHEN %(lng)s IS NOT NULL AND %(lat)s IS NOT NULL THEN
        ST_SetSRID(ST_MakePoint(%(lng)s, %(lat)s), 4326)::geography
    ELSE NULL END,
    %(raw)s,
    %(scrape_run_id)s,
    %(scraped_at)s,
    NOW()
)
ON CONFLICT (place_id) DO UPDATE SET
    company = EXCLUDED.company,
    phone = EXCLUDED.phone,
    website = EXCLUDED.website,
    rating = EXCLUDED.rating,
    reviews = EXCLUDED.reviews,
    type_business = EXCLUDED.type_business,
    address = EXCLUDED.address,
    city = EXCLUDED.city,
    country = EXCLUDED.country,
    location = EXCLUDED.location,
    raw = EXCLUDED.raw,
    scrape_run_id = COALESCE(EXCLUDED.scrape_run_id, companies.scrape_run_id),
    scraped_at = COALESCE(EXCLUDED.scraped_at, companies.scraped_at),
    updated_at = NOW();
"""

_UPSERT_WITHOUT_PLACE_ID = """
INSERT INTO companies (
    company,
    phone,
    website,
    rating,
    reviews,
    type_business,
    address,
    city,
    country,
    location,
    raw,
    scrape_run_id,
    scraped_at,
    updated_at
) VALUES (
    %(company)s,
    %(phone)s,
    %(website)s,
    %(rating)s,
    %(reviews)s,
    %(type_business)s,
    %(address)s,
    %(city)s,
    %(country)s,
    CASE WHEN %(lng)s IS NOT NULL AND %(lat)s IS NOT NULL THEN
        ST_SetSRID(ST_MakePoint(%(lng)s, %(lat)s), 4326)::geography
    ELSE NULL END,
    %(raw)s,
    %(scrape_run_id)s,
    %(scraped_at)s,
    NOW()
)
ON CONFLICT (company, address) WHERE place_id IS NULL DO UPDATE SET
    phone = EXCLUDED.phone,
    website = EXCLUDED.website,
    rating = EXCLUDED.rating,
    reviews = EXCLUDED.reviews,
    type_business = EXCLUDED.type_business,
    city = EXCLUDED.city,
    country = EXCLUDED.country,
    location = EXCLUDED.location,
    raw = EXCLUDED.raw,
    scrape_run_id = COALESCE(EXCLUDED.scrape_run_id, companies.scrape_run_id),
    scraped_at = COALESCE(EXCLUDED.scraped_at, companies.scraped_at),
    updated_at = NOW();
"""


def upsert_company(row: Dict[str, Any]) -> None:
    """Persist a company dictionary, performing an idempotent upsert."""
    params = _prepare_params(row)
    if not params["company"] or not params["address"]:
        raise ValueError("company and address are required for upsert")

    with get_connection() as conn:
        with conn.cursor() as cur:
            if params["place_id"]:
                cur.execute(_UPSERT_WITH_PLACE_ID, params)
            else:
                cur.execute(_UPSERT_WITHOUT_PLACE_ID, params)
        conn.commit()
        logger.debug("Upserted company %s", params["company"])
