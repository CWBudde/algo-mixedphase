set shell := ["bash", "-uc"]

export GOPRIVATE := "github.com/cwbudde"
typst_version := "0.15.0"
paper_source := "docs/paper/paper.typ"
paper_output := "docs/paper/mixed-phase-filter-design-en.pdf"
brief_source := "docs/paper/paper_brief.typ"
brief_output := "docs/paper/mixed-phase-filter-design-brief-en.pdf"

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

# Build every package for the host platform
#
# `go test` links its own main, so a package that fails to build as a command
# still passes the test suite. This recipe is what catches that.
build:
    go build ./...

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
#
# The default ten-minute per-package timeout is not enough for the whole tree at
# once. internal/reference drives the L-BFGS penalty ladder over a 513-bin grid
# for every out-of-window and floor-probe row, and the race detector's
# instrumentation of those inner loops costs roughly a factor of five: the
# package takes 333 s under -race when it runs alone, and exceeds 600 s when it
# competes with the other packages `go test ./...` starts in parallel. The
# timeout is therefore explicit rather than inherited. This recipe is
# deliberately not part of `just ci`.
test-race:
    go test -race -timeout 45m ./...

# Run tests with coverage
test-coverage:
    go test -v -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html

# Enforce the AGENTS.md coverage floor
#
# internal/reference is gated alongside the two public packages: it produces
# every number quoted in docs/ and in the paper, so leaving it ungated would
# exempt exactly the code the reproducibility rules exist to protect.
check-coverage:
    #!/usr/bin/env bash
    set -euo pipefail
    floor=90
    status=0
    workdir="$(mktemp -d)"
    trap 'rm -rf "$workdir"' EXIT
    for pkg in mixedphase graphiceq internal/reference; do
        profile="${workdir}/${pkg//\//-}.out"
        go test "./${pkg}/" -coverprofile="$profile" >/dev/null
        percent="$(go tool cover -func="$profile" | awk '/^total:/ {print $3}' | tr -d '%')"
        if awk "BEGIN{exit !($percent < $floor)}"; then
            echo "FAIL ${pkg}: ${percent}% is below the ${floor}% floor" >&2
            status=1
        else
            echo "ok   ${pkg}: ${percent}%"
        fi
    done
    exit "$status"

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
        -responses docs/reference-response.csv \
        -impulses docs/reference-impulse.csv \
        -sweep docs/reference-delay-sweep.csv \
        -regimes docs/reference-phase-regimes.csv \
        -continuum docs/reference-continuum.csv \
        -continuum-impulses docs/reference-continuum-impulse.csv \
        > docs/reference-results.csv
    go run ./examples/graphiceq > docs/graphiceq-results.csv

# Refresh the wall-clock measurements. Deliberately excluded from `compare` and
# from `compare-check`: these numbers are machine-dependent by nature, and are
# only meaningful alongside the machine and toolchain recorded in the file.
compare-timings trials="5":
    go run ./examples/mixedphase \
        -trials {{ trials }} \
        -timings docs/reference-timings.csv \
        > /dev/null

# Prove the committed comparison artifacts are reproducible
compare-check:
    just compare
    git diff --exit-code -- \
        docs/reference-results.csv \
        docs/reference-response.csv \
        docs/reference-impulse.csv \
        docs/reference-delay-sweep.csv \
        docs/reference-phase-regimes.csv \
        docs/reference-continuum.csv \
        docs/reference-continuum-impulse.csv \
        docs/graphiceq-results.csv \
        docs/MIXED_PHASE_FILTER_DESIGN.md

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

# Build the short companion account. It reads the same committed artifact as the
# full paper, so the two cannot disagree on a number.
paper-brief: paper-check-tools
    ./scripts/run-typst.sh compile --root . \
        --input revision="$(git describe --always --dirty)" \
        {{ brief_source }} {{ brief_output }}

# Rebuild the paper whenever its Typst sources change
paper-watch: paper-check-tools
    ./scripts/run-typst.sh watch --root . \
        --input revision="working-tree" \
        {{ paper_source }} {{ paper_output }}

# Refresh benchmark CSVs explicitly, then rebuild every data-backed chart
paper-refresh:
    just compare
    just paper
    just paper-brief

# Run all checks (formatting, linting, tests, tidiness, reproducibility)
ci: check-formatted build test test-cross-build test-web lint check-tidy check-coverage compare-check

# Clean build artifacts
clean:
    rm -f coverage.out coverage.html {{ paper_output }} {{ brief_output }}

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
