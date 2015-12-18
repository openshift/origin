#!/bin/bash

# GoFmt apparently is changing @ head...

set -o errexit
set -o nounset
set -o pipefail

GO_VERSION=($(go version))

# require go version to be 1.5.1 to pin `go fmt` version to 829cc34.
if [[ -z $(echo "${GO_VERSION[2]}" | grep -E 'go1.5.1') ]]; then
  echo "Unsupported go version '${GO_VERSION}', skipping gofmt."
  exit 0
fi

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"

cd "${OS_ROOT}"

bad_files=$(find_files | xargs gofmt -s -l)
if [[ -n "${bad_files}" ]]; then
  echo "FAILURE: go fmt needs to be run on the following files: "
  echo "${bad_files}"
  echo "Try running 'gofmt -s -d [path]'"
  exit 1
else
  echo "SUCESS: go fmt found no errors"
fi
