.PHONY: build run test lint clean migrate proto docker-up docker-down help

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOVET=$(GOCMD) vet
BINARY_NAME=user-service
BUILD_DIR=bin

# Database
DB_URL ?= postgres://postgres:postgres@localhost:5432/userservice?sslmode=disable
MIGRATE_CMD=migrate

# Proto
BUF_CMD=buf

# Docker
DOCKER_COMPOSE=docker compose

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: Build the application
build:
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server

## run: Run the application
run: build
	@echo "Running..."
	./$(BUILD_DIR)/$(BINARY_NAME)

## dev: Run with hot reload (requires air)
dev:
	@air

## test: Run unit tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -short ./...

## test-integration: Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -race -run Integration ./...

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

## lint: Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...
	goimports -w .

## tidy: Tidy and verify dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy
	$(GOMOD) verify

## proto: Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	$(BUF_CMD) generate

## proto-lint: Lint protobuf files
proto-lint:
	@echo "Linting protobuf files..."
	$(BUF_CMD) lint

## migrate-up: Run database migrations
migrate-up:
	@echo "Running migrations..."
	$(MIGRATE_CMD) -path migrations -database "$(DB_URL)" up

## migrate-down: Rollback database migrations
migrate-down:
	@echo "Rolling back migrations..."
	$(MIGRATE_CMD) -path migrations -database "$(DB_URL)" down 1

## migrate-create: Create a new migration (usage: make migrate-create name=migration_name)
migrate-create:
	@echo "Creating migration..."
	$(MIGRATE_CMD) create -ext sql -dir migrations -seq $(name)

## migrate-force: Force migration version (usage: make migrate-force version=1)
migrate-force:
	@echo "Forcing migration version..."
	$(MIGRATE_CMD) -path migrations -database "$(DB_URL)" force $(version)

## docker-up: Start docker containers
docker-up:
	@echo "Starting docker containers..."
	$(DOCKER_COMPOSE) up -d

## docker-down: Stop docker containers
docker-down:
	@echo "Stopping docker containers..."
	$(DOCKER_COMPOSE) down

## docker-logs: Show docker logs
docker-logs:
	$(DOCKER_COMPOSE) logs -f

## docker-build: Build docker image
docker-build:
	@echo "Building docker image..."
	docker build -t $(BINARY_NAME):latest .

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

## install-tools: Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

## sqlc: Generate sqlc code
sqlc:
	@echo "Generating sqlc code..."
	sqlc generate

## all: Build and test
all: lint vet test build
