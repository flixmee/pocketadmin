#!/usr/bin/env bash

set -euo pipefail
set -m

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DEV_URL="http://localhost:5173/"

backend_pid=""
frontend_pid=""

cleanup() {
    trap - EXIT INT TERM

    if [[ -n "$backend_pid" ]]; then
        kill -- "-$backend_pid" 2>/dev/null || true
    fi

    if [[ -n "$frontend_pid" ]]; then
        kill -- "-$frontend_pid" 2>/dev/null || true
    fi

    wait 2>/dev/null || true
}

trap cleanup EXIT INT TERM

(
    cd "$ROOT_DIR"
    exec go run examples/base/main.go serve
) &
backend_pid=$!

(
    cd "$ROOT_DIR/ui"
    exec npm run dev -- --open "$DEV_URL"
) &
frontend_pid=$!

while kill -0 "$backend_pid" 2>/dev/null && kill -0 "$frontend_pid" 2>/dev/null; do
    sleep 0.2
done

status=0
if ! kill -0 "$backend_pid" 2>/dev/null; then
    wait "$backend_pid" || status=$?
else
    wait "$frontend_pid" || status=$?
fi

exit "$status"
