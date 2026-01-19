.PHONY: all proto build run test test-unit test-acceptance clean deps lint help

# Variables
PROTO_DIR := api/proto
GEN_DIR := api/gen
SWAGGER_DIR := api/swagger
BINARY_NAME := tictactoe-server
GRPC_PORT := 50051
HTTP_PORT := 8080

# Go commands
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOINSTALL := $(GOCMD) install

# Default target
all: deps proto build

# Help target
help:
	@echo "TicTacToe gRPC + REST Server - Available targets:"
	@echo ""
	@echo "  make deps           - Download Go dependencies"
	@echo "  make proto          - Generate Go code from proto files"
	@echo "  make proto-tools    - Install protoc plugins (run once)"
	@echo "  make build          - Build the server binary"
	@echo "  make run            - Run the server (gRPC: $(GRPC_PORT), HTTP: $(HTTP_PORT))"
	@echo "  make test           - Run all tests"
	@echo "  make test-unit      - Run unit tests only"
	@echo "  make test-acceptance- Run acceptance tests only"
	@echo "  make test-load      - Run load tests (100+ concurrent users/games)"
	@echo "  make test-coverage  - Run tests with coverage"
	@echo "  make lint           - Run linter"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make all            - deps + proto + build"
	@echo ""
	@echo "After 'make run', access:"
	@echo "  - Swagger UI: http://localhost:$(HTTP_PORT)/swagger/"
	@echo "  - REST API:   http://localhost:$(HTTP_PORT)/api/v1/..."
	@echo "  - gRPC:       localhost:$(GRPC_PORT)"
	@echo ""

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Install protoc plugins (run once)
proto-tools:
	$(GOINSTALL) google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GOINSTALL) google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	$(GOINSTALL) github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	$(GOINSTALL) github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest

# Generate protobuf code (gRPC + REST gateway + OpenAPI)
proto:
	@mkdir -p $(GEN_DIR)/tictactoe $(SWAGGER_DIR)
	protoc \
		--go_out=$(GEN_DIR)/tictactoe --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR)/tictactoe --go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=$(GEN_DIR)/tictactoe --grpc-gateway_opt=paths=source_relative \
		--openapiv2_out=$(SWAGGER_DIR) \
		-I$(PROTO_DIR) -Ithird_party \
		$(PROTO_DIR)/tictactoe.proto

# Build the server
build:
	$(GOBUILD) -o bin/$(BINARY_NAME) ./cmd/server

# Run the server
run: build
	./bin/$(BINARY_NAME) -grpc-port $(GRPC_PORT) -http-port $(HTTP_PORT)

# Run all tests
test: test-unit test-acceptance

# Run unit tests
test-unit:
	$(GOTEST) -v -race ./internal/game/... ./internal/store/...

# Run acceptance tests
test-acceptance:
	$(GOTEST) -v -race ./tests/...

# Run load tests (100+ concurrent users and games)
test-load:
	$(GOTEST) -v -race -run "TestLoadTest" ./tests/acceptance/

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out ./internal/game/... ./internal/store/... ./tests/...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf $(GEN_DIR)/
	rm -rf $(SWAGGER_DIR)/
	rm -f coverage.out coverage.html

# Development: rebuild and run
dev: build run

# Docker build (optional)
docker-build:
	docker build -t tictactoe-server .

# Docker run (optional)
docker-run:
	docker run -p $(GRPC_PORT):$(GRPC_PORT) -p $(HTTP_PORT):$(HTTP_PORT) tictactoe-server
