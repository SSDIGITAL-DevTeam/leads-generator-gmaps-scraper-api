"""Tests for maps_serp_worker filtering logic."""

import pytest
from unittest.mock import Mock, patch

from maps_serp_worker import run_scrape
from models import CompanyCandidate


@pytest.fixture
def mock_candidates():
    """Create mock company candidates with varying ratings and websites."""
    return [
        CompanyCandidate(
            name="High Rating With Website",
            rating=4.8,
            website="https://example.com",
            review_count=100,
        ),
        CompanyCandidate(
            name="High Rating No Website",
            rating=4.7,
            website=None,
            review_count=50,
        ),
        CompanyCandidate(
            name="Low Rating With Website",
            rating=3.5,
            website="https://low.com",
            review_count=20,
        ),
        CompanyCandidate(
            name="Low Rating No Website",
            rating=3.2,
            website=None,
            review_count=10,
        ),
        CompanyCandidate(
            name="No Rating With Website",
            rating=None,
            website="https://norating.com",
            review_count=5,
        ),
    ]


@patch("maps_serp_worker.send_to_ingest_api")
@patch("maps_serp_worker.parse_serpapi_maps")
@patch("maps_serp_worker.fetch_from_serpapi")
def test_run_scrape_filters_by_min_rating(mock_fetch, mock_parse, mock_send, mock_candidates):
    """Test that candidates below min_rating are filtered out."""
    mock_fetch.return_value = {"local_results": []}
    mock_parse.return_value = mock_candidates
    mock_send.return_value = Mock(status_code=200)

    run_scrape("test query", min_rating=4.5)

    # Should only send candidates with rating >= 4.5
    assert mock_send.called
    payload = mock_send.call_args[0][0]
    items = payload["items"]
    assert len(items) == 2  # Only "High Rating With Website" and "High Rating No Website"
    assert all(item["rating"] >= 4.5 for item in items if item["rating"] is not None)


@patch("maps_serp_worker.send_to_ingest_api")
@patch("maps_serp_worker.parse_serpapi_maps")
@patch("maps_serp_worker.fetch_from_serpapi")
def test_run_scrape_filters_by_require_no_website(mock_fetch, mock_parse, mock_send, mock_candidates):
    """Test that candidates with websites are filtered out when require_no_website=True."""
    mock_fetch.return_value = {"local_results": []}
    mock_parse.return_value = mock_candidates
    mock_send.return_value = Mock(status_code=200)

    run_scrape("test query", require_no_website=True)

    # Should only send candidates without websites
    assert mock_send.called
    payload = mock_send.call_args[0][0]
    items = payload["items"]
    assert len(items) == 2  # Only "High Rating No Website" and "Low Rating No Website"
    assert all(item["website"] is None for item in items)


@patch("maps_serp_worker.send_to_ingest_api")
@patch("maps_serp_worker.parse_serpapi_maps")
@patch("maps_serp_worker.fetch_from_serpapi")
def test_run_scrape_limits_results(mock_fetch, mock_parse, mock_send, mock_candidates):
    """Test that results are limited to the specified limit."""
    mock_fetch.return_value = {"local_results": []}
    mock_parse.return_value = mock_candidates
    mock_send.return_value = Mock(status_code=200)

    run_scrape("test query", limit=2)

    # Should only send first 2 candidates
    assert mock_send.called
    payload = mock_send.call_args[0][0]
    items = payload["items"]
    assert len(items) == 2


@patch("maps_serp_worker.send_to_ingest_api")
@patch("maps_serp_worker.parse_serpapi_maps")
@patch("maps_serp_worker.fetch_from_serpapi")
def test_run_scrape_combined_filters(mock_fetch, mock_parse, mock_send, mock_candidates):
    """Test that all filters work together correctly."""
    mock_fetch.return_value = {"local_results": []}
    mock_parse.return_value = mock_candidates
    mock_send.return_value = Mock(status_code=200)

    run_scrape("test query", min_rating=4.5, require_no_website=True, limit=1)

    # Should filter by rating >= 4.5, no website, and limit to 1
    assert mock_send.called
    payload = mock_send.call_args[0][0]
    items = payload["items"]
    assert len(items) == 1
    assert items[0]["name"] == "High Rating No Website"
    assert items[0]["rating"] >= 4.5
    assert items[0]["website"] is None


@patch("maps_serp_worker.send_to_ingest_api")
@patch("maps_serp_worker.parse_serpapi_maps")
@patch("maps_serp_worker.fetch_from_serpapi")
def test_run_scrape_skips_ingest_when_no_candidates_after_filter(mock_fetch, mock_parse, mock_send, mock_candidates):
    """Test that ingest is skipped when all candidates are filtered out."""
    mock_fetch.return_value = {"local_results": []}
    mock_parse.return_value = mock_candidates
    mock_send.return_value = Mock(status_code=200)

    # Filter that removes all candidates
    run_scrape("test query", min_rating=5.0)

    # Should NOT call send_to_ingest_api
    assert not mock_send.called
