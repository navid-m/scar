#!/usr/bin/env bash

set -uo pipefail

PROGRAM="./scar-dev"
PASS_COUNT=0
FAIL_COUNT=0

pass() {
	printf "\n[pass] %s" "$1"
}

fail() {
	printf "\n[fail] %s" "$1"
}

info() {
	echo "[info] $1"
}

if [ ! -f "$PROGRAM" ]; then
	info "scar-dev not found, running make."

	if ! make; then
		fail "make failed."
		exit 1
	fi
fi

run_sample_tests() {
	while read -r file; do
		if "$PROGRAM" "$file"; then
			pass "$file"
			((PASS_COUNT++))
		else
			fail "$file"
			((FAIL_COUNT++))
		fi
	done < <(
		find ./samples/self -type f -name "*.scar" | sort
	)
}

run_sample_tests

echo
echo "passed: $PASS_COUNT"
echo "failed: $FAIL_COUNT"

if [ "$FAIL_COUNT" -ne 0 ]; then
	exit 1
fi
