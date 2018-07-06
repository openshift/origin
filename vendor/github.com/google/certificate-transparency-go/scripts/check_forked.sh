#!/bin/bash
#
# Checks that source files (.go and .proto) don't import the base
# Go versions of asn1/x509 libraries.
set -eu

check_import() {
  local path="$1"

  local result=$(grep -Hne '\("crypto/x509"\|"encoding/asn1"\)' "$path")
  if [[ ! -z "${result}" ]]; then
    echo "$result - import of base library"
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
