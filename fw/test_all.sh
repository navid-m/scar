#!/usr/bin/env bash

set -uo pipefail

SCAR="./target/debug/scar"

PASS_COUNT=0
FAIL_COUNT=0

pass() {
	echo "[pass] $1"
	PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
	echo "[fail] $1"
	FAIL_COUNT=$((FAIL_COUNT + 1))
}

info() {
	echo "[info] $1"
}

run_stdlib_tests() {
	info "running stdlib tests..."

	if "$SCAR" test lib/std/; then
		pass "lib/std/"
	else
		fail "lib/std/"
	fi
}

run_sample_tests() {
	info "running sample tests..."

	find ./samples -type f -name "*.scar" | while read -r file; do
		if [[ "$file" == *_error.scar ]]; then
			if "$SCAR" "$file" >/dev/null 2>&1; then
				fail "$file (expected error)"
			else
				pass "$file (expected error)"
			fi
		else
			if "$SCAR" "$file"; then
				pass "$file"
			else
				fail "$file"
			fi
		fi
	done
}

run_stdlib_tests
run_sample_tests

echo
echo "passed: $PASS_COUNT"
echo "failed: $FAIL_COUNT"

if [ "$FAIL_COUNT" -ne 0 ]; then
	exit 1
fi
