#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

echo $(go version)

OS_ROOT=$(dirname "${BASH_SOURCE}")/..

cd "${OS_ROOT}"

if go vet ./...; then
  echo "SUCCESS: go vet succeded!"
  exit 0
else
  echo "FAILURE: go vet failed!"
  exit 1
fi
