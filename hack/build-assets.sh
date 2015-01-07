#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

pushd "${OS_ROOT}/assets" > /dev/null
  grunt build
popd > /dev/null

pushd "${OS_ROOT}" > /dev/null
  Godeps/_workspace/bin/go-bindata -nocompress -prefix "assets/dist" -pkg "assets" -o "pkg/assets/bindata.go" -ignore "\\.gitignore" assets/dist/...
popd > /dev/null