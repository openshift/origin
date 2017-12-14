#!/bin/bash
# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


set -o nounset
set -o errexit

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
. "${ROOT}/contrib/hack/utilities.sh" || { echo 'Cannot load utilities.'; exit 1; }

function usage() {
  [[ -n "${1:-}" ]] && { echo "${1}"; echo ; }

  cat <<__EOF__
Usage: coverage.sh [options] packages ...

Runs test code coverage on specified packages and aggregates results.

Supported options:

  --out:  aggregated coverage profile output file name
  --html: aggregated coverage html output file name

__EOF__

  exit 1
}

while [[ $# -ne 0 ]]; do
  case "$1" in
    --out)     OUT="$2";  shift ;;
    --html)    HTML="$2"; shift ;;
    -*)        usage "Unrecognized command line argument $1" ;;
    *)         break;
  esac
  shift
done

[[ -z "${OUT:-}" && -z "${HTML:-}" ]] \
  && usage "Either --out or --html must be specified"

PACKAGES=( ${@+"${@}"} )
OUT=${OUT:-${TMP:=$(mktemp)}}

function cleanup() {
  local tmp="${TMP:-}"
  [[ -n "${tmp}" ]] && rm -f "${tmp}"
}
trap cleanup EXIT

function run-coverage() {
  local packages=(${!1+"${!1}"})
  local package subpackage
  local result=0

  echo 'mode: set' > "${OUT}"

  for package in ${packages[@]+"${packages[@]}"}; do
    for subpackage in $(go list "${package}/..."); do
      local out="$(mktemp)"
      go test -cover "${subpackage}" -coverprofile "${out}" \
        || result=1
      grep -h -v '^mode: ' "${out}" >> "${OUT}"
      rm "${out}"
    done
  done

  [[ ${result} -eq 0 && -n "${HTML:-}" ]] \
    && go tool cover -html "${OUT}" -o "${HTML}"

  return ${result}
}

run-coverage PACKAGES[@]
