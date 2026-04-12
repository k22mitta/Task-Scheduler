BINARY := ./bin/server
CMD     := ./cmd/server

.PHONY: run build test migrate docker-up docker-down

run:
	go run $(CMD)

build:
	go build -o $(BINARY) $(CMD)

test:
	go test ./...

migrate:
	psql "$(DATABASE_URL)" -f migrations/001_create_jobs_table.sql

docker-up:
	docker compose up -d

docker-down:
	docker compose down
