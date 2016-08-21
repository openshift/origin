#!/bin/bash

# This script generates release zips into _output/releases. It requires the openshift/origin-release
# image to be built prior to executing this command via hack/build-base-images.sh.

# NOTE:   only committed code is built.
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

context="${OS_ROOT}/_output/buildenv-context"

# Clean existing output.
rm -rf "${OS_LOCAL_RELEASEPATH}"
rm -rf "${context}"
mkdir -p "${context}"
mkdir -p "${OS_OUTPUT}"

container="$( os::build::environment::create /bin/sh -c "OS_ONLY_BUILD_PLATFORMS=${OS_ONLY_BUILD_PLATFORMS-} make build-cross" )"
trap "os::build::environment::cleanup ${container}" EXIT

# Perform the build and release in Docker.
(
  OS_GIT_TREE_STATE=clean # set this because we will be pulling from git archive
  os::build::get_version_vars
  echo "++ Building release ${OS_GIT_VERSION}"
)
OS_BUILD_ENV_PRESERVE=_output/local/releases os::build::environment::withsource "${container}" "${OS_GIT_COMMIT:-HEAD}"
echo "${OS_GIT_COMMIT}" > "${OS_LOCAL_RELEASEPATH}/.commit"

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
