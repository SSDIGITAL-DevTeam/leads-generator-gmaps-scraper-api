#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATABASE_URL="${DATABASE_URL:-postgres://app:app@localhost:5432/places?sslmode=disable}"
SEED_FILE="${ROOT_DIR}/db/seeds/companies.sample.csv"

if ! command -v psql >/dev/null 2>&1; then
  echo "psql is required to seed the database" >&2
  exit 1
fi

if [ ! -f "${SEED_FILE}" ]; then
  echo "Seed file not found: ${SEED_FILE}" >&2
  exit 1
fi

echo "Seeding companies from ${SEED_FILE}"
psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 <<SQL
\copy companies (company, address, phone, website, rating, reviews, type_business, city, country)
FROM '${SEED_FILE}' WITH (FORMAT csv, HEADER true)
SQL
