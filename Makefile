.PHONY: build run test migrate clean

build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test ./...

migrate:
	@echo "Run migrations manually: psql -d gpu_orchestrator -f migrations/001_initial_schema.sql"

clean:
	rm -rf bin/

deps:
	go mod download
	go mod tidy
