#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

readonly S2I_ROOT=$(dirname "${BASH_SOURCE}")/..

s2i::cleanup() {
  echo
  echo "Complete"
}

readonly img_count="$(docker images | grep -c sti_test/sti-fake || :)"

if [ "${img_count}" != "10" ]; then
    echo "Missing test images, run 'hack/build-test-images.sh' and try again."
    exit 1
fi

trap s2i::cleanup EXIT SIGINT

echo
echo "Running integration tests ..."
echo

S2I_TIMEOUT="-timeout 600s" "${S2I_ROOT}/hack/test-go.sh" test/integration -v -tags 'integration' "${@:1}"
