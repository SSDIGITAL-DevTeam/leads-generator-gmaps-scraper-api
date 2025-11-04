import argparse

import pytest

from src.jobs import run_query


class DummySettings:
    def __init__(self, api_key="test-key", max_pages=3):
        self.google_api_key = api_key
        self.database_url = "postgres://"
        self.worker_port = 9000
        self.max_pages = max_pages


def test_run_query_job_requires_api_key(monkeypatch):
    monkeypatch.setattr(run_query, "get_settings", lambda: DummySettings(api_key=""))

    with pytest.raises(RuntimeError):
        run_query.run_query_job(
            type_business="plumber",
            city="Gotham",
            country="USA",
            min_rating=None,
            max_pages=1,
        )


def test_run_query_job_processes_results(monkeypatch):
    monkeypatch.setattr(run_query, "get_settings", lambda: DummySettings(api_key="abc"))
    monkeypatch.setattr(run_query, "init_pool", lambda: None)
    monkeypatch.setattr(run_query.time, "sleep", lambda _: None)

    calls = []

    def fake_text_search(query, api_key, pagetoken=None):
        if not pagetoken:
            return {"results": [{"place_id": "1"}, {"place_id": "2"}], "next_page_token": "next"}
        return {"results": [{"place_id": "3"}], "next_page_token": None}

    def fake_place_details(place_id, api_key):
        return {
            "place_id": place_id,
            "name": f"Company {place_id}",
            "formatted_address": "Main",
            "formatted_phone_number": "123",
            "website": "https://example.com",
            "rating": 4.5 if place_id != "2" else 3.0,
            "user_ratings_total": 10,
            "types": ["store"],
            "geometry": {"location": {"lng": 10, "lat": 20}},
            "address_components": [],
        }

    def fake_upsert_company(row):
        calls.append(row["place_id"])

    monkeypatch.setattr(run_query.google_places, "text_search", fake_text_search)
    monkeypatch.setattr(run_query.google_places, "place_details", fake_place_details)
    monkeypatch.setattr(run_query, "upsert_company", fake_upsert_company)

    run_query.run_query_job(
        type_business="store",
        city="Gotham",
        country="USA",
        min_rating=4.0,
        max_pages=2,
    )

    assert calls == ["1", "3"]  # place 2 skipped due to rating


def test_run_query_job_requires_query(monkeypatch):
    monkeypatch.setattr(run_query, "get_settings", lambda: DummySettings(api_key="abc"))
    monkeypatch.setattr(run_query, "init_pool", lambda: None)

    with pytest.raises(ValueError):
        run_query.run_query_job(
            type_business=" ",
            city=None,
            country=None,
            min_rating=None,
            max_pages=1,
        )


def test_build_parser_defaults(monkeypatch):
    monkeypatch.setattr(run_query, "get_settings", lambda: DummySettings(api_key="abc", max_pages=7))
    parser = run_query.build_parser()
    args = parser.parse_args(["--type", "store"])
    assert isinstance(parser, argparse.ArgumentParser)
    assert args.type_business == "store"
    assert args.max_pages == 7
