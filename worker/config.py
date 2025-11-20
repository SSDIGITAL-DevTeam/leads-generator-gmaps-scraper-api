"""Configuration helpers for the SerpAPI-powered Stage 1 pipeline.

Environment variables are intentionally the only way to provide credentials:
`SERPAPI_API_KEY` must never be hardcoded because it is a billable key, and
`INGEST_API_URL` lets us point the worker at different Go API deployments.
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from functools import lru_cache
from typing import Optional

try:
    # python-dotenv is optional; load_dotenv() becomes a no-op if missing.
    from dotenv import load_dotenv
except ImportError:  # pragma: no cover - optional dependency
    def load_dotenv() -> None:
        """Placeholder when python-dotenv is not installed."""


class ConfigError(RuntimeError):
    """Raised when mandatory configuration is missing."""


@dataclass(frozen=True)
class Settings:
    serpapi_api_key: str
    ingest_api_url: str
    default_city: Optional[str] = None
    default_country_code: Optional[str] = None


def _get_required_env(name: str) -> str:
    value = os.getenv(name)
    if not value:
        raise ConfigError(f"{name} must be set in the environment for the SerpAPI worker to run.")
    return value


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    """Load, validate and cache worker settings to avoid repeated env lookups."""
    load_dotenv()

    serpapi_api_key = _get_required_env("SERPAPI_API_KEY")
    ingest_api_url = _get_required_env("INGEST_API_URL")
    default_city = os.getenv("DEFAULT_CITY") or None
    default_country_code = os.getenv("DEFAULT_COUNTRY_CODE") or None

    return Settings(
        serpapi_api_key=serpapi_api_key,
        ingest_api_url=ingest_api_url,
        default_city=default_city,
        default_country_code=default_country_code,
    )
