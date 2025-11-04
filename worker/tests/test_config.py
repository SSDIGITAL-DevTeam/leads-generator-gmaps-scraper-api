import os

import pytest

from src.core import config


def setup_function(function):
    config.get_settings.cache_clear()


def teardown_function(function):
    config.get_settings.cache_clear()


def test_get_settings_reads_env(monkeypatch):
    monkeypatch.setenv("GOOGLE_API_KEY", "abc123")
    monkeypatch.setenv("DATABASE_URL", "postgres://user:pass@localhost/db")
    monkeypatch.setenv("WORKER_PORT", "9100")
    monkeypatch.setenv("WORKER_MAX_PAGES", "5")

    settings = config.get_settings()

    assert settings.google_api_key == "abc123"
    assert settings.database_url == "postgres://user:pass@localhost/db"
    assert settings.worker_port == 9100
    assert settings.max_pages == 5


def test_get_settings_warns_when_missing(monkeypatch, caplog):
    monkeypatch.delenv("GOOGLE_API_KEY", raising=False)
    monkeypatch.delenv("DATABASE_URL", raising=False)

    with caplog.at_level("WARNING"):
        settings = config.get_settings()

    assert "DATABASE_URL is not set" in " ".join(caplog.messages)
    assert "GOOGLE_API_KEY is not configured" in " ".join(caplog.messages)
    assert settings.database_url == ""
    assert settings.worker_port == 9000
    assert settings.max_pages == 3
