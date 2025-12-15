.PHONY: build run clean test deps

# Build the application
build:
	@echo "Building ElasticObservability..."
	go build -o elasticobservability ./cmd/main.go

# Run the application
run: build
	@echo "Running ElasticObservability..."
	./elasticobservability -config config.yaml

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f elasticobservability
	rm -rf logs/
	rm -rf outputs/

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Install the application
install:
	@echo "Installing ElasticObservability..."
	go install ./cmd/main.go

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	@echo "Linting code..."
	golangci-lint run ./...

# Create necessary directories
setup:
	@echo "Setting up directories..."
	mkdir -p logs
	mkdir -p outputs
	mkdir -p configs/oneTime
	mkdir -p configs/processedOneTime
	mkdir -p data

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	GOOS=linux GOARCH=amd64 go build -o elasticobservability-linux-amd64 ./cmd/main.go
	GOOS=darwin GOARCH=amd64 go build -o elasticobservability-darwin-amd64 ./cmd/main.go
	GOOS=windows GOARCH=amd64 go build -o elasticobservability-windows-amd64.exe ./cmd/main.go

help:
	@echo "Available targets:"
	@echo "  build      - Build the application"
	@echo "  run        - Build and run the application"
	@echo "  clean      - Remove build artifacts and logs"
	@echo "  test       - Run tests"
	@echo "  deps       - Download and tidy dependencies"
	@echo "  install    - Install the application"
	@echo "  fmt        - Format code"
	@echo "  lint       - Lint code (requires golangci-lint)"
	@echo "  setup      - Create necessary directories"
	@echo "  build-all  - Build for multiple platforms"
