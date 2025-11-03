.PHONY: all run migrate seed test lint docker-up

all: lint test

run:
	go run cmd/app/main.go

migrate:
	chmod +x scripts/migrate.sh
	./scripts/migrate.sh

seed:
	chmod +x scripts/seed.sh && ./scripts/seed.sh 20

test:
	go test -race -cover ./...

lint:
	golangci-lint run --timeout=5m

docker-up:
	docker-compose up --build

docker-down:
	docker-compose down -v