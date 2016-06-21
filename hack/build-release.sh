#!/bin/bash

# This script generates release zips into _output/releases. It requires the openshift/origin-release
# image to be built prior to executing this command via hack/build-base-images.sh.

# NOTE:   only committed code is built.

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::install_errexit

# Go to the top of the tree.
cd "${OS_ROOT}"

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
os::build::environment::withsource "${container}" "${OS_GIT_COMMIT:-HEAD}"
# Get the command output
docker cp "${container}:/go/src/github.com/openshift/origin/_output/local/releases" "${OS_OUTPUT}"
echo "${OS_GIT_COMMIT}" > "${OS_LOCAL_RELEASEPATH}/.commit"

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
