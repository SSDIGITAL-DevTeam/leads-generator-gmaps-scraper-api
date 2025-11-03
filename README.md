# Leads Generator

Backend-oriented monorepo that orchestrates a Go API, a Python worker, and PostgreSQL/PostGIS storage to collect and serve business leads.

## Architecture
- **API (Go/Echo):** REST interface, JWT auth, RBAC-protected admin surfaces, and a rate-limited `/scrape` trigger that proxies jobs to the worker.
- **Worker (Python):** Fetches Google Places data, transforms it, and upserts companies into PostGIS; exposes an internal HTTP endpoint for job intake.
- **Database:** PostgreSQL 16 with PostGIS and pgcrypto enabled for spatial lookups and UUID ids.
- **Deployments:** Docker Compose bundles the API, worker, and database; Make targets wrap common workflows.

## Quick Start (Docker Compose)
1. Copy `.env` template if needed: `cp configs/.env.example .env` (Docker Compose reads host env vars).
2. Start the stack: `make up` (or `docker compose -f deployments/docker/docker-compose.yml up -d`).
3. Apply migrations: `make migrate`.
4. Seed sample data (optional): `make seed`.
5. Verify the API: `curl http://localhost:8080/healthz` -> `{"status":"success","message":"service healthy","data":{"status":"ok"}}`.

## Local Development
- Run API locally with hot reloads: `make api` (requires Go 1.22).
- Run worker locally: `make worker` (requires Python 3.12 + deps `pip install -r worker/requirements.txt`).
- Stop the Compose stack: `make down`.

## Database Operations
- Apply migrations manually: `bash scripts/migrate.sh` (uses `DATABASE_URL`, defaults to local Postgres).
- Seed the companies table from CSV: `bash scripts/seed.sh`.
- Create a backup: `bash scripts/backup_db.sh ./backup.sql`.

## Environment Variables
| Variable | Default | Purpose |
| --- | --- | --- |
| `DATABASE_URL` | `postgres://app:app@db:5432/places?sslmode=disable` | Connection string for Postgres/PostGIS. |
| `JWT_SECRET` | `supersecret` | HMAC secret for JWT signing. |
| `JWT_TTL` | `24h` | Token lifetime (Go duration). |
| `GOOGLE_API_KEY` | `replace_me` | Server key for Google Places API. |
| `WORKER_BASE_URL` | `http://worker:9000` | API -> worker bridge URL. |
| `RATE_LIMIT_SCRAPE` | `5/min` | Global limiter for `/scrape` endpoint. |
| `PORT` | `8080` | External API listen port. |
| `WORKER_PORT` | `9000` | Worker HTTP port. |
| `WORKER_MAX_PAGES` | `3` | Default Places pagination depth for worker jobs. |

## cURL Recipes
> Tip: install [`jq`](https://stedolan.github.io/jq/) to parse responses easily.

1. **Register (optional)**
   ```bash
   curl -X POST "http://localhost:8080/auth/register" \
     -H 'Content-Type: application/json' \
     -d '{"email":"user@example.com","password":"secretpass"}'
   ```
2. **Authenticate and store token**
   ```bash
   TOKEN=$(curl -s -X POST "http://localhost:8080/auth/login" \
     -H 'Content-Type: application/json' \
     -d '{"email":"admin@example.com","password":"secretpass"}' \
     | jq -r '.data.access_token')
   ```
3. **Upload admin CSV**
   ```bash
   curl -X POST "http://localhost:8080/admin/upload-csv" \
     -H "Authorization: Bearer ${TOKEN}" \
     -F "file=@db/seeds/companies.sample.csv"
   ```
4. **Trigger a scrape job**
   ```bash
   curl -X POST "http://localhost:8080/scrape" \
     -H 'Content-Type: application/json' \
     -H "Authorization: Bearer ${TOKEN}" \
     -d '{"type_business":"coffee shop","city":"Jakarta","country":"Indonesia","min_rating":4}'
   ```
5. **List companies (public)**
   ```bash
   curl "http://localhost:8080/companies?city=Jakarta&min_rating=4"
   ```
6. **List companies (admin lens)**
   ```bash
   curl "http://localhost:8080/admin/companies?country=Indonesia" \
     -H "Authorization: Bearer ${TOKEN}"
   ```
7. **Admin user management**
   ```bash
   # Create
   curl -X POST "http://localhost:8080/admin/users" \
     -H "Authorization: Bearer ${TOKEN}" \
     -H 'Content-Type: application/json' \
     -d '{"email":"staff@example.com","password":"changeme","role":"user"}'

   # List
   curl "http://localhost:8080/admin/users" \
     -H "Authorization: Bearer ${TOKEN}"

   # Update
   curl -X PATCH "http://localhost:8080/admin/users/<user-id>" \
     -H "Authorization: Bearer ${TOKEN}" \
     -H 'Content-Type: application/json' \
     -d '{"role":"admin"}'

   # Delete
   curl -X DELETE "http://localhost:8080/admin/users/<user-id>" \
     -H "Authorization: Bearer ${TOKEN}"
   ```

## Google Places Notes
- Respect Google Places quota limits; the worker adds per-item and per-page delays (0.15s / 2.5s) but you may need additional throttling for production use.
- Only use the official Places API as implemented here; scraping raw HTML violates Google terms of service.

## Project Status
This repository ships a runnable skeleton: migrations, repositories, services, handlers, worker ETL, Docker packaging, and OpenAPI spec. Future prompts can flesh out additional business logic and integrations.
