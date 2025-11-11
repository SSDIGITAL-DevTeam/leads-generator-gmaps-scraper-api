"""Application configuration helpers."""

import logging
import os
from dataclasses import dataclass
from functools import lru_cache
from typing import Optional

from dotenv import load_dotenv

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class Settings:
    google_api_key: str
    database_url: str
    worker_port: int = 9000
    max_pages: int = 3
    enrich_callback_url: str = ""
    default_phone_region: Optional[str] = None
    enrich_use_js_renderer: bool = False


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    """Load settings from environment variables with sensible defaults."""
    load_dotenv()

    google_api_key = os.getenv("GOOGLE_API_KEY", "")
    database_url = os.getenv("DATABASE_URL", "")
    worker_port = int(os.getenv("WORKER_PORT", "9000"))
    max_pages = int(os.getenv("WORKER_MAX_PAGES", "3"))
    enrich_callback_url = os.getenv("ENRICH_CALLBACK_URL", "")
    default_phone_region_raw = os.getenv("DEFAULT_PHONE_REGION")
    default_phone_region = default_phone_region_raw.strip().upper() if default_phone_region_raw else None
    enrich_use_js_renderer = os.getenv("ENRICH_USE_JS_RENDERER", "false").lower() in {"1", "true", "yes"}

    if not database_url:
        logger.warning("DATABASE_URL is not set; database operations will fail.")
    if not google_api_key:
        logger.warning("GOOGLE_API_KEY is not configured; Google Places requests will fail.")
    if not enrich_callback_url:
        logger.warning("ENRICH_CALLBACK_URL is not configured; enrichment callbacks will be skipped.")

    return Settings(
        google_api_key=google_api_key,
        database_url=database_url,
        worker_port=worker_port,
        max_pages=max_pages,
        enrich_callback_url=enrich_callback_url,
        default_phone_region=default_phone_region,
        enrich_use_js_renderer=enrich_use_js_renderer,
    )
