import contextlib

import pytest

from src.core import config, db


class DummyCursor:
    def __init__(self, collector):
        self.collector = collector

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        return False

    def execute(self, sql, params):
        self.collector.append((" ".join(sql.split()), params))


class DummyConnection:
    def __init__(self, collector):
        self.collector = collector
        self.commits = 0

    def cursor(self):
        return DummyCursor(self.collector)

    def commit(self):
        self.commits += 1


class DummyPool:
    def __init__(self, connection):
        self.connection = connection
        self.get_called = False
        self.put_called = False

    def getconn(self):
        self.get_called = True
        return self.connection

    def putconn(self, conn):
        assert conn is self.connection
        self.put_called = True


@pytest.fixture(autouse=True)
def reset_pool():
    db._connection_pool = None
    yield
    db._connection_pool = None


def test_init_pool_requires_database_url(monkeypatch):
    config.get_settings.cache_clear()
    monkeypatch.setenv("DATABASE_URL", "")
    monkeypatch.setenv("GOOGLE_API_KEY", "key")

    with pytest.raises(RuntimeError):
        db.init_pool()


def test_init_pool_creates_singleton(monkeypatch):
    class FakePool:
        def __init__(self, minconn, maxconn, dsn, connect_timeout):
            self.args = (minconn, maxconn, dsn, connect_timeout)

    def fake_simple_pool(minconn, maxconn, dsn, connect_timeout):
        return FakePool(minconn, maxconn, dsn, connect_timeout)

    monkeypatch.setenv("DATABASE_URL", "postgres://user:pass@host/db")
    monkeypatch.setenv("GOOGLE_API_KEY", "key")
    config.get_settings.cache_clear()

    monkeypatch.setattr(db.pool, "SimpleConnectionPool", fake_simple_pool)

    pool1 = db.init_pool()
    pool2 = db.init_pool()

    assert pool1 is pool2
    assert pool1.args[0] == 1
    assert pool1.args[2] == "postgres://user:pass@host/db"


def test_get_connection_context_manager(monkeypatch):
    collector = []
    connection = DummyConnection(collector)
    dummy_pool = DummyPool(connection)
    db._connection_pool = dummy_pool

    with db.get_connection() as conn:
        assert conn is connection

    assert dummy_pool.get_called is True
    assert dummy_pool.put_called is True


def test_prepare_params_json_wrapper():
    params = db._prepare_params({
        "place_id": "pid",
        "company": "Acme",
        "phone": "123",
        "website": "https://example.com",
        "rating": 4.5,
        "reviews": 10,
        "type_business": "store",
        "address": "Main St",
        "city": "Gotham",
        "country": "USA",
        "lng": 10,
        "lat": 20,
        "raw": {"foo": "bar"},
    })

    assert params["place_id"] == "pid"
    assert params["raw"].adapted == {"foo": "bar"}


def test_upsert_company_validations():
    with pytest.raises(ValueError):
        db.upsert_company({"address": "Main"})

    with pytest.raises(ValueError):
        db.upsert_company({"company": "Acme"})


def test_upsert_company_executes_statements(monkeypatch):
    collector = []
    connection = DummyConnection(collector)
    dummy_pool = DummyPool(connection)
    db._connection_pool = dummy_pool

    db.upsert_company({"place_id": "pid", "company": "Acme", "address": "Main"})
    db.upsert_company({"company": "Acme", "address": "Main"})

    assert connection.commits == 2
    assert collector[0][0].startswith("INSERT INTO companies ( place_id")
    assert collector[1][0].startswith("INSERT INTO companies ( company")
