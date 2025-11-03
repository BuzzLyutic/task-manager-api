#!/bin/bash
set -euo pipefail

API_URL=${API_URL:-"http://localhost:8080"}
COUNT=${1:-20}

echo "Creating $COUNT tasks..."

for i in $(seq 1 $COUNT); do
    PRIORITY=$((RANDOM % 10 + 1))
    curl -s -X POST "$API_URL/api/tasks" \
        -H "Content-Type: application/json" \
        -H "Idempotency-Key: seed-$i" \
        -d "{\"title\":\"Task $i\",\"priority\":$PRIORITY}" \
        > /dev/null &
done

wait
echo "Done!"