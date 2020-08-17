#!/bin/bash
#
# Checks that source files (.go and .proto) don't import known forks
# of other packages by mistake.
set -eu

check_import() {
  local path="$1"

  local result=$(grep -Hne '\("github.com/gogo/protobuf/proto"\|"golang.org/x/net/context"\)' "$path")
  if [[ ! -z "${result}" ]]; then
    echo "$result - import of forked library"
    return 1
  fi
}

main() {
  if [[ $# -lt 1 ]]; then
    echo "Usage: $0 <path>"
    exit 1
  fi

  local code=0
  while [[ $# -gt 0 ]]; do
    local path="$1"
    if [[ -d "$path" ]]; then
      for f in "$path"/*.{go,proto}; do
        if [[ ! -f "$f" ]]; then
          continue  # Empty glob
        fi
        check_import "$f" || code=1
      done
    else
      check_import "$path" || code=1
    fi
    shift
  done
  exit $code
}

main "$@"
