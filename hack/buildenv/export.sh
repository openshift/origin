#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/buildenv/common.sh"

tar -C "${OS_RELEASES}" -cf - .
