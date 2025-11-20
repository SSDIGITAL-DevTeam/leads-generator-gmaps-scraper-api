# syntax=docker/dockerfile:1
FROM python:3.12-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /app

# lib runtime utk psycopg2-binary & http debug
RUN apt-get update \
    && apt-get install -y --no-install-recommends libpq5 curl \
    && rm -rf /var/lib/apt/lists/*

# cache-friendly (requirements dulu)
COPY requirements.txt ./requirements.txt
RUN pip install --no-cache-dir --default-timeout=120 --retries=5 -r requirements.txt \
    && python -m playwright install --with-deps chromium

# Invalidate cache when source code changes (via git SHA)
ARG GIT_SHA=dev
LABEL git.sha="${GIT_SHA}"

# source belakangan
COPY . .

ENV PYTHONPATH=/app
EXPOSE 9000
CMD ["python", "-m", "src.jobs.run_query_server"]
