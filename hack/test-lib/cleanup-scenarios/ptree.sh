#!/bin/bash
#
# This test case forks itself until a total depth of five, to enable testing os::cleanup::internal::kill_process_tree

set -o errexit
set -o nounset
set -o pipefail

last_depth="${1:-0}"
current_depth="$(( last_depth + 1 ))"
if [[ ${current_depth} -gt 5 ]]; then
	exit 0
fi

( $0 "${current_depth}" )

sleep 10
