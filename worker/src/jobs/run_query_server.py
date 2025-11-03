"""HTTP entrypoint that triggers Google Places jobs."""

import logging
from concurrent.futures import ThreadPoolExecutor
from typing import Any, Dict

from flask import Flask, jsonify, request

from src.core.config import get_settings
from src.jobs.run_query import run_query_job

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s - %(message)s")
logger = logging.getLogger(__name__)

app = Flask(__name__)
_executor = ThreadPoolExecutor(max_workers=4)


@app.route("/scrape", methods=["POST"])
def enqueue_scrape() -> Any:
    payload: Dict[str, Any] = request.get_json(silent=True) or {}
    missing = [field for field in ("type_business", "city", "country") if not payload.get(field)]
    if missing:
        return jsonify({"error": f"missing fields: {', '.join(missing)}"}), 400

    settings = get_settings()
    min_rating = payload.get("min_rating")
    if min_rating is not None:
        try:
            min_rating = float(min_rating)
        except (TypeError, ValueError):
            return jsonify({"error": "min_rating must be numeric"}), 400

    max_pages = payload.get("max_pages") or settings.max_pages
    try:
        max_pages = int(max_pages)
    except (TypeError, ValueError):
        max_pages = settings.max_pages

    job_args = dict(
        type_business=payload.get("type_business"),
        city=payload.get("city"),
        country=payload.get("country"),
        min_rating=min_rating,
        max_pages=max_pages,
    )

    logger.info("Queueing scrape job: %s", job_args)
    _executor.submit(_run_job_safe, job_args)

    return jsonify({"data": {"status": "queued"}}), 200


def _run_job_safe(job_args: Dict[str, Any]) -> None:
    try:
        run_query_job(**job_args)
    except Exception as exc:  # noqa: BLE001
        logger.exception("Scrape job failed: %s", exc)


def main() -> None:
    settings = get_settings()
    app.run(host="0.0.0.0", port=settings.worker_port)


if __name__ == "__main__":
    main()
