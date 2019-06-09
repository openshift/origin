#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

readonly S2I_ROOT=$(dirname "${BASH_SOURCE}")/..

s2i::cleanup() {
  echo
  echo "Complete"
}

trap s2i::cleanup EXIT SIGINT

echo
echo "Running dockerfile integration tests ..."
echo

S2I_TIMEOUT="-timeout 600s" "${S2I_ROOT}/hack/test-go.sh" test/integration/dockerfile -v -tags "integration" "${@:1}"
