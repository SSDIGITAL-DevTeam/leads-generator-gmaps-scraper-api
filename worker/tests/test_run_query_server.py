import types

import pytest

from src.jobs import run_query_server


class DummySettings:
    def __init__(self):
        self.worker_port = 9000
        self.max_pages = 3


@pytest.fixture(autouse=True)
def reset_executor(monkeypatch):
    submitted = {}

    class DummyExecutor:
        def submit(self, fn, args):
            submitted["called"] = True
            submitted["args"] = args

    monkeypatch.setattr(run_query_server, "_executor", DummyExecutor())
    monkeypatch.setattr(run_query_server, "run_scrape", lambda **kwargs: submitted.update(job=kwargs))
    yield submitted


def test_health_endpoint(reset_executor):
    client = run_query_server.app.test_client()
    response = client.get("/healthz")
    assert response.status_code == 200
    assert response.get_json()["status"] == "ok"


def test_enqueue_scrape_validates_payload(reset_executor):
    client = run_query_server.app.test_client()
    assert client.post("/scrape", json={}).status_code == 400
    assert client.post("/scrape", json={"type_business": "store"}).status_code == 400
    assert client.post(
        "/scrape",
        json={"type_business": "store", "city": "Gotham", "country": "USA", "min_rating": "bad"},
    ).status_code == 400


def test_enqueue_scrape_builds_query_and_passes_params(reset_executor):
    client = run_query_server.app.test_client()
    payload = {
        "type_business": "restaurant",
        "city": "Yogyakarta",
        "country": "Indonesia",
        "min_rating": 4.5,
        "limit": 10,
        "require_no_website": True,
    }
    response = client.post("/scrape", json=payload)

    assert response.status_code == 202
    assert reset_executor["called"] is True
    args = reset_executor.get("args")
    assert args["query"] == "restaurant in Yogyakarta, Indonesia"
    assert args["min_rating"] == 4.5
    assert args["limit"] == 10
    assert args["require_no_website"] is True


def test_enqueue_scrape_validates_limit(reset_executor):
    client = run_query_server.app.test_client()
    
    # Invalid limit (non-numeric)
    payload = {"type_business": "store", "city": "Jakarta", "country": "Indonesia", "limit": "bad"}
    assert client.post("/scrape", json=payload).status_code == 400
    
    # Invalid limit (negative)
    payload = {"type_business": "store", "city": "Jakarta", "country": "Indonesia", "limit": -5}
    assert client.post("/scrape", json=payload).status_code == 400
