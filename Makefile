.PHONY: all proto test lint build infra-up infra-down clean

PROTO_DIR := api/proto
GEN_DIR := gen/go

all: proto lint test build

# ------------------------------------------------------------------------------
# Protobuf Compilation Command
# Compiles booking.proto into Go structs and gRPC client/server interfaces
# Requires: protoc, protoc-gen-go, protoc-gen-go-grpc
# ------------------------------------------------------------------------------
proto:
	@echo "==> Compiling Protocol Buffers..."
	@mkdir -p $(GEN_DIR)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/booking/v1/booking.proto
	@echo "==> Protobuf compilation completed successfully."

# ------------------------------------------------------------------------------
# Tooling Prerequisites
# ------------------------------------------------------------------------------
tools:
	@echo "==> Installing protobuf code generation plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.60.3

# ------------------------------------------------------------------------------
# Testing and Quality Assurance
# ------------------------------------------------------------------------------
test:
	@echo "==> Running unit and integration tests..."
	go test -v -race -cover ./...

lint:
	@echo "==> Running linters..."
	golangci-lint run --timeout 5m ./...

# ------------------------------------------------------------------------------
# Local Infrastructure (Docker Compose)
# ------------------------------------------------------------------------------
infra-up:
	@echo "==> Provisioning local infrastructure (PostgreSQL, Redis, Kafka)..."
	docker-compose up -d

infra-down:
	@echo "==> Tearing down local infrastructure..."
	docker-compose down -v

# ------------------------------------------------------------------------------
# Build Binaries
# ------------------------------------------------------------------------------
build:
	@echo "==> Building microservice binaries..."
	mkdir -p bin
	go build -o bin/booking ./cmd/booking

clean:
	@rm -rf bin/ $(GEN_DIR)
