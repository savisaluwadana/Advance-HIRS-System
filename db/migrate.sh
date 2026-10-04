#!/bin/sh
set -eu

if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is required" >&2
  exit 1
fi

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

echo "Applying current Advance HRIS schema..."
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$SCRIPT_DIR/schema.sql"

for migration in "$SCRIPT_DIR"/migrations/*.sql; do
  [ -f "$migration" ] || continue
  echo "Applying $(basename "$migration")..."
  psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$migration"
done

echo "Database schema is ready."
