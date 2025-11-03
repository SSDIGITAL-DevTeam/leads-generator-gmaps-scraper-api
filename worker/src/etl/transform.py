"""Utilities for transforming Google Places responses into database rows."""

import logging
from typing import Any, Dict, Iterable, Optional, Tuple

logger = logging.getLogger(__name__)

_IGNORE_TYPES = {"point_of_interest", "establishment", "political", "premise"}


def parse_city_country(address_components: Iterable[Dict[str, Any]]) -> Tuple[Optional[str], Optional[str]]:
    city = None
    country = None
    for component in address_components or []:
        types = set(component.get("types", []))
        if "locality" in types or "administrative_area_level_2" in types:
            city = component.get("long_name")
        if "country" in types:
            country = component.get("long_name")
    return city, country


def _extract_primary_type(types: Iterable[str]) -> Optional[str]:
    for type_name in types or []:
        if type_name not in _IGNORE_TYPES:
            return type_name
    return None


def to_company_row(result: Dict[str, Any], fallback_city: Optional[str], fallback_country: Optional[str]) -> Dict[str, Any]:
    geometry = result.get("geometry", {}).get("location", {})
    city, country = parse_city_country(result.get("address_components", []))
    if not city:
        city = fallback_city
    if not country:
        country = fallback_country

    return {
        "place_id": result.get("place_id"),
        "company": result.get("name"),
        "phone": result.get("formatted_phone_number"),
        "website": result.get("website"),
        "rating": result.get("rating"),
        "reviews": result.get("user_ratings_total"),
        "type_business": _extract_primary_type(result.get("types", [])),
        "address": result.get("formatted_address"),
        "city": city,
        "country": country,
        "lng": geometry.get("lng"),
        "lat": geometry.get("lat"),
        "raw": result,
    }
