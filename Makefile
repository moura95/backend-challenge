include .env

# Database migrations
migrate-up:
	migrate -database ${DB_SOURCE} -path internal/infra/database/migrations up

migrate-down:
	migrate -database ${DB_SOURCE} -path internal/infra/database/migrations down --all

# Docker Compose
up:
	docker compose up --build -d

down:
	docker compose down --volumes && docker volume prune -f

# Docker Build
build:
	docker build --build-arg ENV_FILE=.env -f Dockerfile -t backend-challenge:latest .

# Code generation
sqlc:
	sqlc generate

swag:
	swag init -g cmd/main.go -d . --parseInternal --parseDependency
# Local development
run:
	go run cmd/main.go

start:
	make up

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

clean:
	go clean
	rm -f coverage.out coverage.html

# Complete workflows
ci:
	make fmt
	make vet
	make test

.PHONY: migrate-up migrate-down  up  down build sqlc swag run start test test-domain test-coverage fmt vet clean ci
