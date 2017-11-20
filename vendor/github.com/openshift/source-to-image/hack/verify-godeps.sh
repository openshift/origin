#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

S2I_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${S2I_ROOT}/hack/common.sh"

s2i::build::build_binaries tools/godepchecker
_output/local/go/bin/godepchecker
