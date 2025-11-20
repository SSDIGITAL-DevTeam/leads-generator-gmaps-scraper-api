"""Core data models shared by the SerpAPI ingestion pipeline."""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, Optional


@dataclass(slots=True)
class CompanyCandidate:
    """Normalized snapshot of a business returned by SerpAPI Google Maps."""

    name: str
    address: Optional[str] = None
    phone: Optional[str] = None
    website: Optional[str] = None
    latitude: Optional[float] = None
    longitude: Optional[float] = None
    rating: Optional[float] = None
    review_count: Optional[int] = None
    source: str = "serpapi_google_maps"
    raw_snapshot: Optional[Dict[str, Any]] = field(default=None, repr=False)
