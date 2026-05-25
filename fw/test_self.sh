#!/usr/bin/env bash

set -uo pipefail

PROGRAM="./scar-dev"
PASS_COUNT=0
FAIL_COUNT=0
EXHAUSTIVE=false

for arg in "$@"; do
	case "$arg" in
		-exhaustive|-e) EXHAUSTIVE=true ;;
		*) echo "unknown flag: $arg"; exit 1 ;;
	esac
done

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

run_stdlib_tests() {
	while read -r module; do
		if "$PROGRAM" test "$module" 2>&1; then
			pass "$module"
			((PASS_COUNT++))
		else
			fail "$module"
			((FAIL_COUNT++))
		fi
	done < <(
		find ./lib/std -mindepth 2 -maxdepth 2 -name "mod.scar" | sort
	)
}

run_sample_tests

if $EXHAUSTIVE; then
	run_stdlib_tests
fi

echo
echo "passed: $PASS_COUNT"
echo "failed: $FAIL_COUNT"

if [ "$FAIL_COUNT" -ne 0 ]; then
	exit 1
fi
