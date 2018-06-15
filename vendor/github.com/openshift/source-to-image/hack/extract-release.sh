#!/bin/bash

# This script extracts a valid release tar into _output/releases. It requires hack/build-release.sh
# to have been executed

set -o errexit
set -o nounset
set -o pipefail

S2I_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${S2I_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${S2I_ROOT}"

# Copy the linux release archives release back to the local _output/local/bin/linux/amd64 directory.
# TODO: support different OS's?
s2i::build::detect_local_release_tars "linux-amd64"

mkdir -p "${S2I_OUTPUT_BINPATH}/linux/amd64"
tar mxzf "${S2I_PRIMARY_RELEASE_TAR}" -C "${S2I_OUTPUT_BINPATH}/linux/amd64"

s2i::build::make_binary_symlinks
