#!/bin/bash
set -euo pipefail

echo "==> Running Go Tests..."

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Run tests with race detector and coverage
echo -e "${YELLOW}Running unit tests...${NC}"
go test -race -coverprofile=coverage.out -covermode=atomic ./internal/... ./pkg/... -v

# Run integration tests
echo -e "${YELLOW}Running integration tests...${NC}"
go test -race -tags=integration ./tests/... -v -timeout=5m

# Display coverage
echo -e "${YELLOW}Coverage report:${NC}"
go tool cover -func=coverage.out | tail -n 1

# Check coverage threshold
COVERAGE=$(go tool cover -func=coverage.out | tail -n 1 | awk '{print $3}' | sed 's/%//')
THRESHOLD=80

if (( $(echo "$COVERAGE < $THRESHOLD" | bc -l) )); then
    echo -e "${RED}❌ Coverage ${COVERAGE}% is below threshold ${THRESHOLD}%${NC}"
    exit 1
else
    echo -e "${GREEN}✅ Coverage ${COVERAGE}% meets threshold ${THRESHOLD}%${NC}"
fi

# Generate HTML coverage report
echo -e "${YELLOW}Generating HTML coverage report...${NC}"
go tool cover -html=coverage.out -o coverage.html
echo -e "${GREEN}Coverage report saved to coverage.html${NC}"

echo -e "${GREEN}==> All tests passed!${NC}"