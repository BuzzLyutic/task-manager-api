.PHONY: run migrate test docker-up

run:
	go run cmd/app/main.go

migrate:
	chmod +x scripts/migrate.sh
	./scripts/migrate.sh

test:
	go test -race -cover ./...

docker-up:
	docker-compose up --build

lint:
	golangci-lint run
