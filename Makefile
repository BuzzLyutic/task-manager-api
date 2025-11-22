.PHONY: all run migrate seed test test-unit test-integration test-coverage lint docker-up

all: lint test

run:
	go run cmd/app/main.go

migrate:
	chmod +x scripts/migrate.sh
	./scripts/migrate.sh

seed:
	chmod +x scripts/seed.sh && ./scripts/seed.sh 20

test:
	chmod +x scripts/test.sh && ./scripts/test.sh

test-unit:
	go test -v -race ./internal/... ./pkg/...

test-integration:
	go test -v -race -timeout=5m ./tests/...

test-coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-verbose:
	go test -v -race -cover ./... -timeout=5m

lint:
	golangci-lint run --timeout=5m

docker-up:
	docker-compose up --build

docker-down:
	docker-compose down -v

clean:
	rm -f coverage.out coverage.html
	go clean -testcache