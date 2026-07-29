set shell := ["bash", "-uc"]

export GOPRIVATE := "github.com/cwbudde"

# Default recipe - show available commands
default:
    @just --list

# Format all code using treefmt
fmt:
    treefmt --allow-missing-formatter

# Check if code is formatted correctly
check-formatted:
    treefmt --allow-missing-formatter --fail-on-change

# Run linters
lint:
    GOCACHE="${GOCACHE:-/tmp/gocache}" GOMODCACHE="${GOMODCACHE:-/tmp/gomodcache}" GOLANGCI_LINT_CACHE="${GOLANGCI_LINT_CACHE:-/tmp/golangci-lint-cache}" golangci-lint run --timeout=2m ./...

# Run linters with auto-fix
lint-fix:
    GOCACHE="${GOCACHE:-/tmp/gocache}" GOMODCACHE="${GOMODCACHE:-/tmp/gomodcache}" GOLANGCI_LINT_CACHE="${GOLANGCI_LINT_CACHE:-/tmp/golangci-lint-cache}" golangci-lint run --fix --timeout=2m ./...

# Ensure go.mod is tidy
check-tidy:
    go mod tidy
    git diff --exit-code go.mod go.sum

# Run all tests
test:
    go test -v ./...

# Run tests with race detector
test-race:
    go test -race ./...

# Run tests with coverage
test-coverage:
    go test -v -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

# Run benchmarks
bench:
    go test -run=^$ -bench=. -benchmem ./...

# Run CI benchmark subset (fast, machine-readable) covering the design entry points
bench-ci:
    go test -run='^$' -bench='BenchmarkDesign' -benchmem -count=1 ./mixedphase/ ./graphiceq/

# Regenerate the comparison CSVs quoted in the docs
compare:
    go run ./examples/mixedphase
    go run ./examples/graphiceq

# Run all checks (formatting, linting, tests, tidiness)
ci: check-formatted test lint check-tidy

# Clean build artifacts
clean:
    rm -f coverage.out coverage.html

# Build the Mixed Phase Lab Go/WASM assets
web-wasm:
    ./web/build-wasm.sh

# Run the local Mixed Phase Lab
web-demo port="8787": web-wasm
    @echo "Serving Mixed Phase Lab at http://localhost:{{port}}"
    python3 -m http.server {{port}} -d web

fix:
    just lint-fix
    just fmt
