#!/bin/bash

# GoFmt apparently is changing @ head...

set -o errexit
set -o nounset
set -o pipefail

GO_VERSION=($(go version))

if [[ -z $(echo "${GO_VERSION[2]}" | grep -E 'go1.6') && -z "${FORCE_VERIFY-}"  ]]; then
  echo "Unknown go version '${GO_VERSION}', skipping gofmt." >&2
  exit 0
fi

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"

cd "${OS_ROOT}"

bad_files=$(find_files | xargs gofmt -s -l)
if [[ -n "${bad_files}" ]]; then
  echo "!!! gofmt needs to be run on the following files: " >&2
  echo "${bad_files}"
  echo "Try running 'gofmt -s -d [path]'" >&2
  echo "Or autocorrect with 'hack/verify-gofmt.sh | xargs -n 1 gofmt -s -w'" >&2
  exit 1
fi
