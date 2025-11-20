"""Worker entrypoint that bridges SerpAPI Google Maps results with the Go ingest API."""

from __future__ import annotations

import argparse
import logging
from dataclasses import asdict
from typing import Dict, List, Optional

import requests

from .config import ConfigError, get_settings
from .models import CompanyCandidate
from .serp_client import fetch_from_serpapi, parse_serpapi_maps

logger = logging.getLogger(__name__)
logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s - %(message)s")


def to_ingest_payload(candidates: List[CompanyCandidate]) -> Dict[str, List[Dict[str, object]]]:
    """Convert CompanyCandidate objects into the JSON payload accepted by the Go ingest API."""
    items: List[Dict[str, object]] = []
    for candidate in candidates:
        entry = asdict(candidate)
        if entry.get("raw_snapshot") is None:
            entry["raw_snapshot"] = {}
        items.append(entry)
    return {"items": items}


def send_to_ingest_api(payload: Dict[str, object]):
    """POST normalized candidates to the Go ingest endpoint.

    Defensive behavior:
    - Uses a requests.Session configured with retries for transient failures.
    - On persistent failure writes payload to worker/data/failed/*.json for later replay.
    - Returns the Response on success, Response on non-2xx, or None on network failure.
    """
    from pathlib import Path
    from requests.adapters import HTTPAdapter
    from urllib3.util.retry import Retry
    import time, json

    settings = get_settings()
    ingest_url = settings.ingest_api_url
    logger.info("INGEST_API_URL=%s", ingest_url)

    # Configure session with retries for networking/5xx errors
    session = requests.Session()
    retries = Retry(
        total=3,
        backoff_factor=1,
        status_forcelist=(500, 502, 503, 504),
        allowed_methods=("POST", "GET"),
    )
    session.mount("http://", HTTPAdapter(max_retries=retries))
    session.mount("https://", HTTPAdapter(max_retries=retries))

    try:
        response = session.post(ingest_url, json=payload, timeout=10)
    except requests.RequestException as exc:
        logger.error("Failed to call ingest API: %s", exc)
        # Persist payload for later replay
        failed_dir = Path(__file__).resolve().parent.joinpath("data", "failed")
        try:
            failed_dir.mkdir(parents=True, exist_ok=True)
            fname = failed_dir.joinpath(f"failed-{int(time.time())}.json")
            with fname.open("w", encoding="utf-8") as fh:
                json.dump(payload, fh, ensure_ascii=False, indent=2)
            logger.info("Saved failed payload to %s", str(fname))
        except Exception as e:
            logger.error("Failed to save failed payload to disk: %s", e)
        # Do not raise here so worker continues; caller can inspect return value
        return None

    # If remote returned non-2xx save payload too for inspection
    if not (200 <= response.status_code < 300):
        logger.error(
            "Ingest API returned non-2xx status (%s): %s", response.status_code, response.text[:500]
        )
        try:
            failed_dir = Path(__file__).resolve().parent.joinpath("data", "failed")
            failed_dir.mkdir(parents=True, exist_ok=True)
            fname = failed_dir.joinpath(f"failed-{int(time.time())}.json")
            with fname.open("w", encoding="utf-8") as fh:
                json.dump({"status": response.status_code, "text": response.text, "payload": payload}, fh, ensure_ascii=False, indent=2)
            logger.info("Saved failed payload (non-2xx) to %s", str(fname))
        except Exception as e:
            logger.error("Failed to save non-2xx payload: %s", e)
        # Return the response so caller can inspect status/text if needed
        return response

    return response


def run_scrape(query: str, ll: Optional[str] = None) -> None:
    """Full pipeline: fetch from SerpAPI, normalize candidates, and send to the ingest API.

    Callers should dedupe or cache identical queries upstream to avoid burning through SerpAPI
    credits and to stay within SerpAPI's rate limits. We still log query-level stats here so
    schedulers can aggregate usage.
    """
    logger.info("Starting Stage 1 scrape for query=%s ll=%s", query, ll)
    raw_data = fetch_from_serpapi(query, ll)
    candidates = parse_serpapi_maps(raw_data)
    logger.info("Parsed %s company candidates from SerpAPI response.", len(candidates))

    if not candidates:
        logger.warning("No candidates found for query=%s. Skipping ingest.", query)
        return

    payload = to_ingest_payload(candidates)
    response = send_to_ingest_api(payload)
    logger.info("Posted %s candidates to ingest API. status=%s", len(candidates), response.status_code)


def _parse_cli_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="SerpAPI Google Maps ingestion worker.")
    parser.add_argument("query", help="Human friendly query, e.g. 'coffee shop in Yogyakarta'")
    parser.add_argument(
        "--ll",
        help="Optional SerpAPI ll parameter in the form '@lat,lng,zoom' (e.g. '@-7.7956,110.3695,13z')",
        default=None,
    )
    return parser.parse_args()


if __name__ == "__main__":
    args = _parse_cli_args()
    try:
        run_scrape(args.query, args.ll)
    except ConfigError as exc:
        logger.error("Configuration error: %s", exc)
        raise SystemExit(2) from exc
    except Exception as exc:  # pragma: no cover - CLI fallback
        logger.error("SerpAPI worker failed: %s", exc, exc_info=True)
        raise SystemExit(1) from exc
