#!/bin/bash

# Build all cross compile targets and the base binaries

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

OS_BUILD_PLATFORMS=("${OS_COMPILE_PLATFORMS[@]}")
os::build::build_binaries "${OS_COMPILE_TARGETS[@]}"

OS_BUILD_PLATFORMS=("${OS_CROSS_COMPILE_PLATFORMS[@]}")
os::build::build_binaries "${OS_CROSS_COMPILE_TARGETS[@]}"

OS_RELEASE_ARCHIVES="${OS_OUTPUT}/releases"
os::build::place_bins
