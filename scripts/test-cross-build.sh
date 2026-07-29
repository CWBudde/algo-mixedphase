#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT HUP INT TERM

if [ -z "${GOCACHE:-}" ]; then
	GOCACHE=/tmp/algo-mixedphase-gocache
	export GOCACHE
fi

cd "$ROOT_DIR"

TEST_PATTERN='^TestIterative(CrossBuildDeterminism|Conditioning)$'
go test ./mixedphase -run "$TEST_PATTERN" -count=1

GOROOT_DIR=$(go env GOROOT)
WASM_RUNNER=
for candidate in \
	"$GOROOT_DIR/lib/wasm/go_js_wasm_exec" \
	"$GOROOT_DIR/misc/wasm/go_js_wasm_exec"; do
	if [ -x "$candidate" ]; then
		WASM_RUNNER=$candidate
		break
	fi
done

if [ -z "$WASM_RUNNER" ]; then
	echo "go_js_wasm_exec not found under GOROOT" >&2
	exit 1
fi

GOOS=js GOARCH=wasm go test -c -o "$TEMP_DIR/mixedphase.test.wasm" ./mixedphase

# Keep the Node/WASM process environment small. wasm_exec rejects an
# oversized environment before running the test.
env -i \
	PATH="$GOROOT_DIR/bin:/usr/local/bin:/usr/bin:/bin" \
	"$WASM_RUNNER" \
	"$TEMP_DIR/mixedphase.test.wasm" \
	-test.run "$TEST_PATTERN" \
	-test.count 1
