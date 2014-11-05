#!/bin/bash

set -e

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

pushd "${OS_ROOT}/assets" > /dev/null
  bundle exec grunt serve
popd > /dev/null