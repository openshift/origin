#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

echo $(go version)

if ! which golint &>/dev/null; then
  echo "Unable to detect 'golint' package"
  echo "To install it, run: 'go get github.com/golang/lint/golint'"
  exit 1
fi

S2I_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${S2I_ROOT}/hack/util.sh"

cd "${S2I_ROOT}"

bad_files=$(s2i::util::find_files | xargs -n1 golint)

if [[ -n "${bad_files}" ]]; then
  echo "!!! golint detected the following problems:"
  echo "${bad_files}"
  exit 1
fi
