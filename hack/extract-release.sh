#!/bin/bash

# This script extracts a valid release tar into _output/releases. It requires hack/build-release.sh
# to have been executed
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

# Copy the release archives release back to the local _output/local/bin/... directories.
# NOTE: On Mac and Windows you must pass WARN=1 in order to extract the output.
os::build::detect_local_release_tars $(os::build::host_platform_friendly)

mkdir -p "${OS_OUTPUT_BINPATH}/$(os::build::host_platform)"
os::build::extract_tar "${OS_PRIMARY_RELEASE_TAR}" "${OS_OUTPUT_BINPATH}/$(os::build::host_platform)"
os::build::extract_tar "${OS_CLIENT_RELEASE_TAR}" "${OS_OUTPUT_BINPATH}/$(os::build::host_platform)"
os::build::extract_tar "${OS_IMAGE_RELEASE_TAR}" "${OS_OUTPUT_BINPATH}/$(os::build::host_platform)"

os::build::make_openshift_binary_symlinks
