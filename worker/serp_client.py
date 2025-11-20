"""SerpAPI Google Maps helpers for the Stage 1 ingestion pipeline."""

from __future__ import annotations

import logging
import random
import time
from typing import Any, Dict, Iterable, List, Optional

from serpapi import GoogleSearch

from config import get_settings
from models import CompanyCandidate

logger = logging.getLogger(__name__)

RETRY_LIMIT = 2
RETRY_DELAY_SECONDS = 1.2


def build_serpapi_params(query: str, ll: Optional[str] = None) -> Dict[str, Any]:
    """Construct SerpAPI request parameters for the Google Maps engine."""
    if not query or not query.strip():
        raise ValueError("Query must be provided for SerpAPI lookups.")

    settings = get_settings()
    params: Dict[str, Any] = {
        "engine": "google_maps",
        "q": query.strip(),
        "api_key": settings.serpapi_api_key,
        "type": "search",
    }
    if ll:
        params["ll"] = ll
    return params


def fetch_from_serpapi(query: str, ll: Optional[str] = None) -> Dict[str, Any]:
    """Call SerpAPI Google Maps and return the raw JSON response with retry logic.

    SerpAPI charges per request, so upstream schedulers should dedupe identical
    queries and cache recent responses; we still log request attempts to track
    usage volume for alerting and post-hoc audits.
    """
    params = build_serpapi_params(query, ll)

    attempt = 0
    while True:
        attempt += 1
        try:
            logger.info("Calling SerpAPI (attempt %s) for query=%s ll=%s", attempt, query, ll)
            search = GoogleSearch(params)
            data = search.get_dict()
            if not data:
                raise ValueError("SerpAPI returned an empty payload.")
            if "error" in data:
                message = data.get("error") or data
                raise RuntimeError(f"SerpAPI returned an error response: {message}")
            return data
        except Exception as exc:  # pragma: no cover - network calls hard to test
            logger.warning("SerpAPI request failed (attempt %s/%s): %s", attempt, RETRY_LIMIT + 1, exc)
            if attempt > RETRY_LIMIT:
                logger.error("SerpAPI request exhausted retries for query=%s", query)
                raise
            sleep_for = RETRY_DELAY_SECONDS + random.uniform(0, 0.8)
            time.sleep(sleep_for)


def parse_serpapi_maps(data: Optional[Dict[str, Any]]) -> List[CompanyCandidate]:
    """Extract SerpAPI local/place results into normalized CompanyCandidate objects."""
    if not data:
        return []

    candidates: List[CompanyCandidate] = []
    items = _extract_items(data)

    if not items:
        logger.warning(
            "SerpAPI response missing local_results iterable. keys=%s preview=%s",
            list(data.keys())[:10],
            str(data.get("local_results"))[:200],
        )
        place_results = data.get("place_results")
        if isinstance(place_results, list):
            items = place_results
        elif isinstance(place_results, dict):
            items = [place_results]

    for raw in items:
        if not isinstance(raw, dict):
            continue

        name = (raw.get("title") or raw.get("name") or "").strip()
        if not name:
            continue

        gps = raw.get("gps_coordinates") or {}
        latitude = _safe_float(gps.get("latitude"))
        longitude = _safe_float(gps.get("longitude"))
        rating = _safe_float(raw.get("rating"))
        review_count = _safe_int(raw.get("reviews_count") or raw.get("reviews"))

        candidate = CompanyCandidate(
            name=name,
            address=_strip_or_none(raw.get("address")),
            phone=_strip_or_none(raw.get("phone")),
            website=_strip_or_none(raw.get("website")),
            latitude=latitude,
            longitude=longitude,
            rating=rating,
            review_count=review_count,
            raw_snapshot=raw,
        )
        candidates.append(candidate)

    return candidates


def _extract_items(data: Dict[str, Any]) -> Iterable[Any]:
    """SerpAPI sometimes returns local_results as a list or nested dict."""
    local_results = data.get("local_results")
    if isinstance(local_results, list):
        return local_results
    if isinstance(local_results, dict):
        logger.debug("local_results is dict with keys: %s", list(local_results.keys())[:10])
        candidate_lists = [
            local_results.get("places"),
            local_results.get("results"),
            local_results.get("local_results"),
        ]
        for maybe in candidate_lists:
            if isinstance(maybe, list):
                return maybe
    return []


def _strip_or_none(value: Any) -> Optional[str]:
    if value is None:
        return None
    value_str = str(value).strip()
    return value_str or None


def _safe_float(value: Any) -> Optional[float]:
    try:
        if value is None:
            return None
        return float(value)
    except (TypeError, ValueError):
        return None


def _safe_int(value: Any) -> Optional[int]:
    if value is None:
        return None
    if isinstance(value, (int, float)):
        return int(value)

    if isinstance(value, str):
        digits = "".join(ch for ch in value if ch.isdigit())
        if digits:
            try:
                return int(digits)
            except ValueError:
                return None
    return None
