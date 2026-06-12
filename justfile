# servd development tasks

# List available recipes
default:
    @just --list

# Run golangci-lint and go vet
lint:
    golangci-lint run ./...
    go vet ./...

# Run all tests
test:
    go test ./...

# Run tests with race detector and coverage
test-race:
    go test -race -cover ./...

# Build the servd binary into ./bin
build:
    go build -o bin/servd ./cmd/servd

# Install servd to GOPATH/bin
install:
    go install ./cmd/servd

# Format source files
fmt:
    gofmt -w .

# Lint, test, and build
all: lint test build

# Remove build artifacts
clean:
    rm -rf bin
