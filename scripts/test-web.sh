#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
TEMP_DIR=$(mktemp -d)
HTTP_PORT=${WEB_TEST_PORT:-18787}
DEBUG_PORT=${WEB_DEBUG_PORT:-19222}
SERVER_PID=
CHROME_PID=

cleanup() {
	if [ -n "$CHROME_PID" ]; then
		kill "$CHROME_PID" 2>/dev/null || true
	fi
	if [ -n "$SERVER_PID" ]; then
		kill "$SERVER_PID" 2>/dev/null || true
	fi
	rm -rf "$TEMP_DIR"
}
trap cleanup EXIT HUP INT TERM

if command -v google-chrome >/dev/null 2>&1; then
	CHROME=google-chrome
elif command -v google-chrome-stable >/dev/null 2>&1; then
	CHROME=google-chrome-stable
elif command -v chromium >/dev/null 2>&1; then
	CHROME=chromium
else
	echo "Chrome or Chromium is required for the browser smoke test" >&2
	exit 1
fi

cd "$ROOT_DIR"

python3 -m http.server "$HTTP_PORT" -d web >"$TEMP_DIR/server.log" 2>&1 &
SERVER_PID=$!

"$CHROME" \
	--headless=new \
	--no-sandbox \
	--disable-background-timer-throttling \
	--disable-dev-shm-usage \
	--disable-gpu \
	--disable-renderer-backgrounding \
	--remote-debugging-port="$DEBUG_PORT" \
	--user-data-dir="$TEMP_DIR/chrome" \
	about:blank >"$TEMP_DIR/chrome.log" 2>&1 &
CHROME_PID=$!

attempt=0
until curl --fail --silent "http://127.0.0.1:$HTTP_PORT/" >/dev/null &&
	curl --fail --silent "http://127.0.0.1:$DEBUG_PORT/json/version" >/dev/null; do
	attempt=$((attempt + 1))
	if [ "$attempt" -ge 100 ]; then
		echo "browser smoke-test services did not start" >&2
		cat "$TEMP_DIR/server.log" >&2
		cat "$TEMP_DIR/chrome.log" >&2
		exit 1
	fi
	sleep 0.1
done

node scripts/web-smoke.mjs \
	"$DEBUG_PORT" \
	"http://127.0.0.1:$HTTP_PORT/?preset=daga-interpolation&length=129&cutoff=0.08&aMethod=iterative&aDelay=8&aTolerance=1&aIterations=12&bMethod=interpolation&bDelay=8&bTolerance=1&bIterations=12"
