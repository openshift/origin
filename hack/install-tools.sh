#!/bin/bash

set -e

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

GO_VERSION=($(go version))
echo "Detected go version: $(go version)"

if [[ ${GO_VERSION[2]} == "go1.4"* ]]; then
  go get golang.org/x/tools/cmd/cover
else
  go get code.google.com/p/go.tools/cmd/cover
fi
