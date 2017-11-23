#!/bin/bash

# Build all cross compile targets and the base binaries

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
S2I_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${S2I_ROOT}/hack/common.sh"
source "${S2I_ROOT}/hack/util.sh"
s2i::log::install_errexit

# Build the primary for all platforms
S2I_BUILD_PLATFORMS=("${S2I_CROSS_COMPILE_PLATFORMS[@]}")
s2i::build::build_binaries "${S2I_CROSS_COMPILE_TARGETS[@]}"

# Make the primary release.
S2I_RELEASE_ARCHIVE="source-to-image"
S2I_BUILD_PLATFORMS=("${S2I_CROSS_COMPILE_PLATFORMS[@]}")
s2i::build::place_bins "${S2I_CROSS_COMPILE_BINARIES[@]}"

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
