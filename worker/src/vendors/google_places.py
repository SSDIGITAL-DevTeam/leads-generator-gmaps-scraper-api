"""Client utilities for the Google Places API."""

import logging
from typing import Any, Dict, Optional

import requests

logger = logging.getLogger(__name__)
_SESSION = requests.Session()
_BASE_URL = "https://maps.googleapis.com/maps/api/place"


class GooglePlacesError(RuntimeError):
    """Raised when the Places API returns a non-successful response."""


def text_search(query: str, api_key: str, pagetoken: Optional[str] = None) -> Dict[str, Any]:
    params = {"query": query, "key": api_key}
    if pagetoken:
        params["pagetoken"] = pagetoken
    response = _SESSION.get(f"{_BASE_URL}/textsearch/json", params=params, timeout=10)
    response.raise_for_status()
    payload = response.json()
    status = payload.get("status")
    if status not in {"OK", "ZERO_RESULTS"}:
        logger.error("text_search failed: status=%s, error_message=%s", status, payload.get("error_message"))
        raise GooglePlacesError(payload.get("error_message") or status)
    return payload


def place_details(place_id: str, api_key: str) -> Dict[str, Any]:
    fields = "place_id,name,formatted_address,formatted_phone_number,geometry,website,rating,user_ratings_total,types,address_components"
    params = {"place_id": place_id, "key": api_key, "fields": fields}
    response = _SESSION.get(f"{_BASE_URL}/details/json", params=params, timeout=10)
    response.raise_for_status()
    payload = response.json()
    status = payload.get("status")
    if status not in {"OK", "ZERO_RESULTS"}:
        logger.error("place_details failed: status=%s, error_message=%s", status, payload.get("error_message"))
        raise GooglePlacesError(payload.get("error_message") or status)
    return payload.get("result", {})
