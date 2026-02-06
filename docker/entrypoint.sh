#!/bin/sh
set -e

echo "╔══════════════════════════════════════╗"
echo "║         CineVault v0.3.0             ║"
echo "║    Waiting for services...           ║"
echo "╚══════════════════════════════════════╝"

# Wait for PostgreSQL
until PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -c '\q' 2>/dev/null; do
  echo "Waiting for PostgreSQL at $DB_HOST:${DB_PORT:-5432}..."
  sleep 2
done
echo "PostgreSQL is ready."

# Auto-apply migrations in order
echo "Applying migrations..."
if [ -d /app/migrations ]; then
  for f in /app/migrations/001_initial_schema.up.sql \
           /app/migrations/002_phase2_schema.up.sql \
           /app/migrations/003_phase3_schema.up.sql \
           /app/migrations/004_library_settings.up.sql; do
    if [ -f "$f" ]; then
      echo "  -> $(basename "$f")"
      PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -U "$DB_USER" -d "$DB_NAME" -f "$f" 2>&1 \
        | grep -v "already exists\|duplicate key\|ERROR\|NOTICE" || true
    fi
  done
fi
echo "Migrations complete."

echo "Starting CineVault..."
exec "$@"
