#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..

# Go to the top of the tree.
cd "${OS_ROOT}"

OS_TEST_TAGS="extended" \
  OS_TEST_PACKAGE="test/extended" \
  OS_TEST_NAMESPACE="extended-test" \
  hack/test-integration.sh $@
