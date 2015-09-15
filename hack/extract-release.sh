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

# Copy the linux release archives release back to the local _output/local/go/bin directory.
# TODO: support different OS's?
os::build::detect_local_release_tars "linux"

mkdir -p "${OS_LOCAL_BINPATH}"
tar mxzf "${OS_PRIMARY_RELEASE_TAR}" -C "${OS_LOCAL_BINPATH}"
tar mxzf "${OS_IMAGE_RELEASE_TAR}" -C "${OS_LOCAL_BINPATH}"

os::build::make_openshift_binary_symlinks