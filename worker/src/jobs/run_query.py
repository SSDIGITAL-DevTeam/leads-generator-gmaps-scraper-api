"""CLI job to fetch Google Places data and persist it."""

import argparse
import logging
import time
import uuid
from datetime import datetime, timezone
from typing import Optional

from src.core.config import get_settings
from src.core.db import init_pool, upsert_company
from src.etl.transform import to_company_row
from src.vendors import google_places

logger = logging.getLogger(__name__)


def run_query_job(
    *,
    type_business: str,
    city: Optional[str],
    country: Optional[str],
    min_rating: Optional[float],
    max_pages: int,
) -> None:
    settings = get_settings()
    api_key = settings.google_api_key
    if not api_key:
        raise RuntimeError("GOOGLE_API_KEY is required")

    init_pool()

    parts = [type_business, city, country]
    query = " ".join(filter(None, parts)).strip()
    if not query:
        raise ValueError("Query parameters are empty")

    logger.info("Running Places text search for query=%s", query)

    scrape_run_id = uuid.uuid4()
    scraped_at = datetime.now(timezone.utc)
    logger.info("Assigned scrape_run_id=%s", scrape_run_id)

    page_token = None
    processed_pages = 0

    while processed_pages < max_pages:
        response = google_places.text_search(query=query, api_key=api_key, pagetoken=page_token)
        results = response.get("results", [])
        logger.info("Fetched %d results on page %d", len(results), processed_pages + 1)

        for result in results:
            place_id = result.get("place_id")
            if not place_id:
                logger.debug("Skipping result without place_id: %s", result)
                continue

            try:
                details = google_places.place_details(place_id=place_id, api_key=api_key)
            except Exception as exc:  # noqa: BLE001
                logger.warning("Failed to fetch details for %s: %s", place_id, exc)
                continue

            row = to_company_row(details, fallback_city=city, fallback_country=country)
            row["scrape_run_id"] = scrape_run_id
            row["scraped_at"] = scraped_at
            rating_val = row.get("rating")
            if min_rating is not None and rating_val is not None:
                try:
                    if float(rating_val) < float(min_rating):
                        logger.debug("Skipping %s due to rating %.2f", place_id, float(rating_val))
                        continue
                except (TypeError, ValueError):
                    logger.debug("Unable to parse rating for %s", place_id)

            try:
                upsert_company(row)
            except Exception as exc:  # noqa: BLE001
                logger.error("Failed to upsert %s: %s", place_id, exc)
                continue

            time.sleep(0.15)

        processed_pages += 1
        page_token = response.get("next_page_token")
        if not page_token:
            break
        time.sleep(2.5)

    logger.info("Completed run: pages_processed=%d", processed_pages)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Run Google Places ETL job")
    parser.add_argument("--type", dest="type_business", required=True, help="Business type to search")
    parser.add_argument("--city", dest="city", help="City filter")
    parser.add_argument("--country", dest="country", help="Country filter")
    parser.add_argument("--min-rating", dest="min_rating", type=float, help="Minimum rating to persist")
    parser.add_argument(
        "--max-pages",
        dest="max_pages",
        type=int,
        default=get_settings().max_pages,
        help="Maximum number of result pages to process",
    )
    return parser


def main() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s - %(message)s")
    parser = build_parser()
    args = parser.parse_args()

    run_query_job(
        type_business=args.type_business,
        city=args.city,
        country=args.country,
        min_rating=args.min_rating,
        max_pages=args.max_pages,
    )


if __name__ == "__main__":
    main()
