#!/bin/bash

set -e

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

GRUNT_PORT=${GRUNT_PORT:-9000}
GRUNT_HOSTNAME=${GRUNT_HOSTNAME:-localhost}

pushd "${OS_ROOT}/assets" > /dev/null
  bundle exec grunt serve --port=$GRUNT_PORT --hostname=$GRUNT_HOSTNAME
popd > /dev/null
