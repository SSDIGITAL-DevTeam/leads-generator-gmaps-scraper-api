#!/usr/bin/env bash
set -euo pipefail

DATABASE_URL="${DATABASE_URL:-postgres://app:app@localhost:5432/places?sslmode=disable}"
OUTPUT_PATH="${1:-backup_$(date +%Y%m%d_%H%M%S).sql}"

if ! command -v pg_dump >/dev/null 2>&1; then
  echo "pg_dump is required to create backups" >&2
  exit 1
fi

echo "Writing backup to ${OUTPUT_PATH}"
pg_dump "${DATABASE_URL}" >"${OUTPUT_PATH}"
