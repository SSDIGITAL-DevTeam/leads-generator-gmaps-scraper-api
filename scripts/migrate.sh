#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DATABASE_URL="${DATABASE_URL:-postgres://app:app@localhost:5432/places?sslmode=disable}"

if ! command -v psql >/dev/null 2>&1; then
  echo "psql is required to run migrations" >&2
  exit 1
fi

echo "Applying migrations to ${DATABASE_URL}"
for migration in "${ROOT_DIR}"/db/migrations/*.sql; do
  [ -e "$migration" ] || continue
  echo "-> ${migration}"
  psql "${DATABASE_URL}" -v ON_ERROR_STOP=1 -f "$migration"
done
