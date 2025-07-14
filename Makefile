include .env

migrate-up:
	migrate -database ${DB_SOURCE} -path internal/infra/database/migrations up

migrate-down:
	migrate -database ${DB_SOURCE} -path internal/infra/database/migrations down --all

down:
	docker compose -f deployments/docker-compose/docker-compose.yml down --volumes && docker volume prune -f

up:
	docker compose -f deployments/docker-compose/docker-compose.yml up -d

sqlc:
	sqlc generate

run:
	go run cmd/main.go

start:
	make up
	sleep 5
	make migrate-up
	go run cmd/main.go

restart:
	make down
	make up
	sleep 10
	make migrate-up
	go run cmd/main.go

swag:
	swag init -g cmd/main.go

# Testing
test:
	go test -v ./...

test-domain:
	go test -v ./internal/domain/...

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Code Quality
fmt:
	go fmt ./...

vet:
	go vet ./...

# Docker
build:
	docker build -t backend-challenge:latest -f deployments/build/Dockerfile .

# Complete workflows
ci:
	make fmt
	make vet
	make test
	make build

.PHONY: migrate-up migrate-down down up sqlc start run restart swag test test-domain test-coverage fmt vet build ci