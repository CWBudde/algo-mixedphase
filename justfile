set shell := ["bash", "-uc"]

export GOPRIVATE := "github.com/cwbudde"
typst_version := "0.15.0"
paper_source := "docs/paper/paper.typ"
paper_output := "docs/paper/mixed-phase-filter-design-en.pdf"

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

# Check the iterative-design golden result in native and JavaScript/WASM builds
test-cross-build:
    ./scripts/test-cross-build.sh

# Check URL state and exercise the WebAssembly lab in a headless browser
test-web: web-wasm
    node --test web/lab-state.test.mjs
    ./scripts/test-web.sh

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

# Regenerate the committed comparison CSVs quoted in the docs
compare:
    go run ./examples/mixedphase \
        -document docs/MIXED_PHASE_FILTER_DESIGN.md \
        > docs/reference-results.csv
    go run ./examples/graphiceq > docs/graphiceq-results.csv

# Verify the pinned paper compiler is available
paper-check-tools:
    @command -v typst >/dev/null || { echo "typst {{ typst_version }} is required"; exit 1; }
    @version="$(./scripts/run-typst.sh --version | awk '{print $2}')"; \
        test "$version" = "{{ typst_version }}" || { \
            echo "typst {{ typst_version }} is required (found $version)"; \
            exit 1; \
        }

# Build the revised English paper from committed inputs
paper: paper-check-tools
    ./scripts/run-typst.sh compile --root . \
        --input revision="$(git describe --always --dirty)" \
        {{ paper_source }} {{ paper_output }}

# Rebuild the paper whenever its Typst sources change
paper-watch: paper-check-tools
    ./scripts/run-typst.sh watch --root . \
        --input revision="working-tree" \
        {{ paper_source }} {{ paper_output }}

# Run all checks (formatting, linting, tests, tidiness)
ci: check-formatted test test-cross-build test-web lint check-tidy

# Clean build artifacts
clean:
    rm -f coverage.out coverage.html {{ paper_output }}

# Build the Mixed Phase Lab Go/WASM assets
web-wasm:
    ./web/build-wasm.sh

# Run the local Mixed Phase Lab
web-demo port="8787": web-wasm
    @echo "Serving Mixed Phase Lab at http://localhost:{{ port }}"
    python3 -m http.server {{ port }} -d web

fix:
    just lint-fix
    just fmt
