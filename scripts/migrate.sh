#!/bin/bash
set -euo pipefail

DB_URL=${DATABASE_URL:-"postgres://user:pass@localhost:5432/taskdb?sslmode=disable"}

echo "Running migrations..."
migrate -path ./migrations -database "$DB_URL" up
echo "Migrations completed!"
