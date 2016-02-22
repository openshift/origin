#!/bin/bash

# This script extracts a valid release tar into _output/releases. It requires hack/build-release.sh
# to have been executed

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

# Copy the linux release archives release back to the local _output/local/bin/linux/amd64 directory.
# TODO: support different OS's?
os::build::detect_local_release_tars $(os::build::host_platform_friendly)

mkdir -p "${OS_OUTPUT_BINPATH}/linux/amd64"
os::build::extract_tar "${OS_PRIMARY_RELEASE_TAR}" "${OS_OUTPUT_BINPATH}/linux/amd64"
os::build::extract_tar "${OS_CLIENT_RELEASE_TAR}" "${OS_OUTPUT_BINPATH}/linux/amd64"
os::build::extract_tar "${OS_IMAGE_RELEASE_TAR}" "${OS_OUTPUT_BINPATH}/linux/amd64"

os::build::make_openshift_binary_symlinks
