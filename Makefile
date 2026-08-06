.PHONY: run seed migrate-up migrate-down test test-race build

run:
	go run ./cmd/api

seed:
	go run ./cmd/seed

migrate-up:
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down:
	migrate -path migrations -database "$$DATABASE_URL" down

test:
	go test ./...

test-race:
	go test -race ./...

build:
	go build -o bin/api ./cmd/api
