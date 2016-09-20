#!/bin/bash

# This script sets up a go workspace locally and builds all go components.
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

# only works on Linux for now, all other platforms must build binaries themselves
if [[ -z "$@" ]]; then
  if [[ "${OS_RELEASE:-}" != "n" ]] && \
    os::build::detect_local_release_tars $(os::build::host_platform_friendly) >/dev/null; then
    platform=$(os::build::host_platform)
    echo "++ Using release artifacts from ${OS_RELEASE_COMMIT} for ${platform} instead of building"
    mkdir -p "${OS_OUTPUT_BINPATH}/${platform}"
    os::build::extract_tar "${OS_PRIMARY_RELEASE_TAR}" "${OS_OUTPUT_BINPATH}/${platform}"
    os::build::extract_tar "${OS_CLIENT_RELEASE_TAR}" "${OS_OUTPUT_BINPATH}/${platform}"
    os::build::extract_tar "${OS_IMAGE_RELEASE_TAR}" "${OS_OUTPUT_BINPATH}/${platform}"

    os::build::make_openshift_binary_symlinks

    ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
  fi
fi

os::build::build_binaries "$@"
os::build::place_bins "$@"
os::build::make_openshift_binary_symlinks

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
