#!/bin/bash

# This script generates release zips into _output/releases. It requires the openshift/origin-release
# image to be built prior to executing this command via hack/build-base-images.sh.

# NOTE:   only committed code is built.

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# Go to the top of the tree.
cd "${OS_ROOT}"

context="${OS_ROOT}/_output/buildenv-context"

# Clean existing output.
rm -rf "${OS_LOCAL_RELEASEPATH}"
rm -rf "${context}"
mkdir -p "${context}"
mkdir -p "${OS_OUTPUT}"

# Generate version definitions.
# You can commit a specific version by specifying OS_GIT_COMMIT="" prior to build
os::build::get_version_vars
os::build::save_version_vars "${context}/os-version-defs"

echo "++ Building release ${OS_GIT_VERSION}"

# Create the input archive.
git archive --format=tar -o "${context}/archive.tar" "${OS_GIT_COMMIT}"
tar -rf "${context}/archive.tar" -C "${context}" os-version-defs
gzip -f "${context}/archive.tar"

# Perform the build and release in Docker.
cat "${context}/archive.tar.gz" | docker run -i --cidfile="${context}/cid" openshift/origin-release
docker cp $(cat ${context}/cid):/go/src/github.com/openshift/origin/_output/local/releases "${OS_OUTPUT}"
echo "${OS_GIT_COMMIT}" > "${OS_LOCAL_RELEASEPATH}/.commit"

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
