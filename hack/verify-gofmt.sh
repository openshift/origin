#!/bin/bash

# GoFmt apparently is changing @ head...

set -o errexit
set -o nounset
set -o pipefail

GO_VERSION=($(go version))

GO_VERSION=($(go version))

if [[ -z $(echo "${GO_VERSION[2]}" | grep -E 'go1.4') ]]; then
  echo "Unknown go version '${GO_VERSION}', skipping gofmt."
  exit 0
fi

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

cd "${OS_ROOT}"

find_files() {
  find . -not \( \
      \( \
        -wholename './output' \
        -o -wholename './_output' \
        -o -wholename './.git' \
        -o -wholename './release' \
        -o -wholename './pkg/assets/bindata.go' \
        -o -wholename './pkg/assets/*/bindata.go' \
        -o -wholename './target' \
        -o -wholename '*/third_party/*' \
        -o -wholename '*/Godeps/*' \
      \) -prune \
    \) -name '*.go'
}

bad_files=$(find_files | xargs gofmt -s -l)
if [[ -n "${bad_files}" ]]; then
  echo "!!! gofmt needs to be run on the following files: "
  echo "${bad_files}"
  echo "Try running 'gofmt -s -d [path]'"
  exit 1
fi
