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
    monkeypatch.setattr(run_query_server, "run_query_job", lambda **kwargs: submitted.update(job=kwargs))
    monkeypatch.setattr(run_query_server, "get_settings", lambda: DummySettings())
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


def test_enqueue_scrape_coerces_defaults(reset_executor):
    client = run_query_server.app.test_client()
    payload = {"type_business": "store", "city": "Gotham", "country": "USA", "max_pages": "invalid"}
    response = client.post("/scrape", json=payload)

    assert response.status_code == 200
    assert reset_executor["called"] is True
    assert reset_executor.get("args") == {
        "type_business": "store",
        "city": "Gotham",
        "country": "USA",
        "min_rating": None,
        "max_pages": 3,
    }
