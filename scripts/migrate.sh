#!/bin/bash
set -euo pipefail

DB_URL=${DATABASE_URL:-"postgres://user:pass@localhost:5432/taskdb?sslmode=disable"}
MIGRATE=$(go env GOPATH)/bin/migrate

echo "Running migrations..."
$MIGRATE -path ./migrations -database "$DB_URL" up
echo "Migrations completed!"
