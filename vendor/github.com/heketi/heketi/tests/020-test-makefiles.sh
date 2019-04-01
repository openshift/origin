#!/bin/bash

# Check for Makefile syntax/style.
# Currently only checks for consistent
# use of tabs (instead of spaces) for indentation.

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"

BASE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

FOUND=$(find "${BASE_DIR}" -name Makefile | \
	grep -v ./vendor | \
	xargs grep -n -E "^[[:space:]]* [[:space:]]*[^[:space:]]")

if [[ -n "${FOUND}" ]]; then
	echo "Found spaces in Makefiles:"
	echo "${FOUND}"
	exit 1
fi

exit 0
