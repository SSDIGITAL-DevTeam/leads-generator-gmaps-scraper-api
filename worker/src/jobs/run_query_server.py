"""HTTP entrypoint that triggers Google Places jobs (Cloud Run friendly)."""

from __future__ import annotations

import logging
import os
from concurrent.futures import ThreadPoolExecutor
from typing import Any, Dict

from flask import Flask, jsonify, request

from src.core.config import get_settings
from src.core.site_enricher import SiteEnricher, post_enrich_result
from src.jobs.run_query import run_query_job

# ---------- Logging ----------
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(levelname)s %(name)s - %(message)s",
)
logger = logging.getLogger(__name__)

# ---------- App & executor ----------
app = Flask(__name__)
_executor = ThreadPoolExecutor(max_workers=4)

# ---------- Routes ----------


@app.get("/")
def root() -> Any:
    """Simple root to avoid 404 on GET /"""
    return "ok", 200


@app.get("/healthz")
def healthcheck() -> Any:
    """
    Lightweight health endpoint.
    Tidak memaksa koneksi DB—cukup baca settings (ENV-based).
    """
    settings = get_settings()
    return (
        jsonify(
            {
                "status": "ok",
                "worker_port_config": getattr(settings, "worker_port", None),
                "revision": os.getenv("K_REVISION", "unknown"),
                "region": os.getenv("X_GOOGLE_RUNTIMEREGION", "unknown"),
            }
        ),
        200,
    )


@app.post("/scrape")
def enqueue_scrape() -> Any:
    """
    Enqueue a Google Places scraping job.
    Required JSON fields: type_business, city, country
    Optional: min_rating (float), max_pages (int)
    """
    payload: Dict[str, Any] = request.get_json(silent=True) or {}

    required = ("type_business", "city", "country")
    missing = [f for f in required if not payload.get(f)]
    if missing:
        return jsonify({"error": f"missing fields: {', '.join(missing)}"}), 400

    settings = get_settings()

    # min_rating (optional -> float)
    min_rating_raw = payload.get("min_rating")
    min_rating = None
    if min_rating_raw is not None:
        try:
            min_rating = float(min_rating_raw)
        except (TypeError, ValueError):
            return jsonify({"error": "min_rating must be numeric"}), 400

    # max_pages (optional -> int, default from settings)
    max_pages_raw = payload.get("max_pages", settings.max_pages)
    try:
        max_pages = int(max_pages_raw)
    except (TypeError, ValueError):
        max_pages = settings.max_pages

    job_args = dict(
        type_business=payload["type_business"],
        city=payload["city"],
        country=payload["country"],
        min_rating=min_rating,
        max_pages=max_pages,
    )

    logger.info("Queueing scrape job: %s", job_args)
    _executor.submit(_run_job_safe, job_args)

    # 202 Accepted lebih tepat untuk async enqueue
    return jsonify({"data": {"status": "queued"}}), 202


@app.post("/enrich")
def enrich_website() -> Any:
    """Enrich a website by crawling a limited set of pages for contact data."""

    payload: Dict[str, Any] = request.get_json(silent=True) or {}
    company_id = payload.get("company_id")
    website = payload.get("website")

    if not company_id or not website:
        return jsonify({"error": "company_id and website are required"}), 400

    try:
        with SiteEnricher(website) as enricher:
            enrichment = enricher.enrich()
    except ValueError as exc:
        return jsonify({"error": str(exc)}), 400
    except Exception as exc:  # noqa: BLE001
        logger.exception("Enrichment failed for %s: %s", website, exc)
        return jsonify({"error": "enrichment failed"}), 500

    response_payload = {"company_id": company_id, **enrichment}

    # Fire-and-forget callback to the Go API, failures are logged inside helper.
    post_enrich_result(company_id, enrichment)

    return jsonify({"data": response_payload}), 200


# ---------- Internals ----------


def _run_job_safe(job_args: Dict[str, Any]) -> None:
    try:
        run_query_job(**job_args)
    except Exception as exc:  # noqa: BLE001
        logger.exception("Scrape job failed: %s", exc)


def main() -> None:
    """
    Wajib: Cloud Run meng-inject env PORT (umumnya 8080).
    Selalu pakai PORT ini. Jika tidak tersedia (local/dev), fallback 8080.
    """
    env_port = os.getenv("PORT")
    logger.info("[BOOT] ENV PORT=%s", env_port)

    port = int(env_port or 8080)
    logger.info("[BOOT] Binding on 0.0.0.0:%d", port)

    # Penting: listen di 0.0.0.0 dan port di atas
    app.run(host="0.0.0.0", port=port)


if __name__ == "__main__":
    main()
