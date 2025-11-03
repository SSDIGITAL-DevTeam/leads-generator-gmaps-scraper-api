"""Application configuration helpers."""

import logging
import os
from dataclasses import dataclass
from functools import lru_cache

from dotenv import load_dotenv

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class Settings:
    google_api_key: str
    database_url: str
    worker_port: int = 9000
    max_pages: int = 3


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    """Load settings from environment variables with sensible defaults."""
    load_dotenv()

    google_api_key = os.getenv("GOOGLE_API_KEY", "")
    database_url = os.getenv("DATABASE_URL", "")
    worker_port = int(os.getenv("WORKER_PORT", "9000"))
    max_pages = int(os.getenv("WORKER_MAX_PAGES", "3"))

    if not database_url:
        logger.warning("DATABASE_URL is not set; database operations will fail.")
    if not google_api_key:
        logger.warning("GOOGLE_API_KEY is not configured; Google Places requests will fail.")

    return Settings(
        google_api_key=google_api_key,
        database_url=database_url,
        worker_port=worker_port,
        max_pages=max_pages,
    )
