.PHONY: run seed migrate-up migrate-down test test-race build swagger

run:
	go run ./cmd/api

seed:
	go run ./cmd/seed

swagger:
	swag init -g cmd/api/main.go -o docs

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
