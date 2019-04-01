#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${0}")" && pwd)"

cd "${SCRIPT_DIR}/../client/api/python" || exit 1

if ! command -v tox &>/dev/null; then
	echo "warning: tox not installed... skipping check" >&2
	exit 0
fi

exec tox -e flake8
