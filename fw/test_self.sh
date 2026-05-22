#!/usr/bin/env bash

set -uo pipefail

PROGRAM="./scar-out/main"

PASS_COUNT=0
FAIL_COUNT=0

pass() {
	printf "\n[pass] $1"
	PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
	printf "\n[fail] $1"
	FAIL_COUNT=$((FAIL_COUNT + 1))
}

info() {
	echo "[info] $1"
}

run_sample_tests() {
	find ./samples/self -type f -name "*.scar" | sort | while read -r file; do
		if "$PROGRAM" "$file"; then
			pass "$file"
		else
			fail "$file"
		fi
	done
}

run_sample_tests

echo
echo "passed: $PASS_COUNT"
echo "failed: $FAIL_COUNT"

if [ "$FAIL_COUNT" -ne 0 ]; then
	exit 1
fi
