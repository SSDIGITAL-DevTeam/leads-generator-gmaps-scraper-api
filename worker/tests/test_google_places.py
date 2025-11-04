import pytest

from src.vendors import google_places


class DummyResponse:
    def __init__(self, status_code=200, payload=None):
        self.status_code = status_code
        self._payload = payload or {}
        self.requested = {}

    def raise_for_status(self):
        if self.status_code >= 400:
            raise RuntimeError("http error")

    def json(self):
        return self._payload


class DummySession:
    def __init__(self):
        self.calls = []

    def get(self, url, params=None, timeout=None):
        self.calls.append((url, params, timeout))
        return self.response


@pytest.fixture(autouse=True)
def patch_session(monkeypatch):
    session = DummySession()
    monkeypatch.setattr(google_places, "_SESSION", session)
    return session


def test_text_search_success(patch_session):
    patch_session.response = DummyResponse(payload={"status": "OK", "results": []})
    payload = google_places.text_search("pizza", "key")
    assert payload["status"] == "OK"
    url, params, timeout = patch_session.calls[0]
    assert "textsearch" in url
    assert params["query"] == "pizza"
    assert timeout == 10


def test_text_search_error_status(patch_session):
    patch_session.response = DummyResponse(payload={"status": "INVALID_REQUEST", "error_message": "bad"})
    with pytest.raises(google_places.GooglePlacesError):
        google_places.text_search("pizza", "key")


def test_place_details_success(patch_session):
    patch_session.response = DummyResponse(payload={"status": "OK", "result": {"name": "Acme"}})
    result = google_places.place_details("pid", "key")
    assert result["name"] == "Acme"


def test_place_details_error(patch_session):
    patch_session.response = DummyResponse(payload={"status": "OVER_QUERY_LIMIT", "error_message": "limit"})
    with pytest.raises(google_places.GooglePlacesError):
        google_places.place_details("pid", "key")
