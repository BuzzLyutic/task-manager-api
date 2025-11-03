#!/bin/sh
set -euo pipefail

DB_URL=${DATABASE_URL:-"postgres://user:pass@localhost:5432/taskdb?sslmode=disable"}

echo "Waiting for database..."
until psql "$DB_URL" -c '\q' 2>/dev/null; do
  echo "PostgreSQL is unavailable - sleeping"
  sleep 1
done

echo "Database is ready!"

echo "Running migrations..."
migrate -path ./migrations -database "$DB_URL" up
echo "Migrations completed!"
