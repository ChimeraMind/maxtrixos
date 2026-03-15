#!/bin/bash
# test.sh - Run all tests in the matrixOS repository.
set -eu

REPO_ROOT="$(cd "$(dirname "${0}")/.." && pwd)"
BATS="${BATS:-bats}"

failed=0

# --- Go tests (vector/) ---
echo "=== Go tests ==="
if command -v go &>/dev/null; then
    (cd "${REPO_ROOT}/vector" && go test ./...)
    echo
else
    echo "SKIP: go not found in PATH"
    echo
fi

# --- Bats tests (shell libraries) ---
echo "=== Bats tests ==="
if command -v "${BATS}" &>/dev/null; then
    bats_files=()
    while IFS= read -r -d '' f; do
        bats_files+=("$f")
    done < <(find "${REPO_ROOT}/build" "${REPO_ROOT}/vector" "${REPO_ROOT}/dev" \
        -name '*_test.bats' -print0 2>/dev/null | sort -z)

    if [ ${#bats_files[@]} -eq 0 ]; then
        echo "No .bats test files found."
    else
        "${BATS}" "${bats_files[@]}"
    fi
    echo
else
    echo "SKIP: ${BATS} not found in PATH (set BATS env var to override)"
    echo
fi

exit ${failed}
