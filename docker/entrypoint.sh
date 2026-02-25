#!/bin/sh
set -e

DB_HOST="${DB_HOST:-db}"
DB_PORT="${DB_PORT:-5432}"

echo "CineVault v2 starting..."
echo "Waiting for database at ${DB_HOST}:${DB_PORT}..."

i=0
while ! nc -z "$DB_HOST" "$DB_PORT" 2>/dev/null; do
    i=$((i + 1))
    if [ "$i" -ge 30 ]; then
        echo "Database not ready after 30 attempts, starting anyway..."
        break
    fi
    sleep 2
done

echo "Database ready, starting application..."
exec /app/cinevault "$@"
