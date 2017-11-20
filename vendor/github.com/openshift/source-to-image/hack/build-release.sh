#!/bin/bash

# This script generates release zips into _output/releases. It requires the openshift/sti-release
# image to be built prior to executing this command.

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
S2I_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${S2I_ROOT}/hack/common.sh"
source "${S2I_ROOT}/hack/util.sh"
s2i::log::install_errexit

# Go to the top of the tree.
cd "${S2I_ROOT}"

# Build the images
echo "++ Building openshift/sti-release"
docker build -q --tag openshift/sti-release "${S2I_ROOT}/images/release"

context="${S2I_ROOT}/_output/buildenv-context"

# Clean existing output.
rm -rf "${S2I_LOCAL_RELEASEPATH}"
rm -rf "${context}"
mkdir -p "${context}"
mkdir -p "${S2I_OUTPUT}"

# Generate version definitions.
# You can commit a specific version by specifying S2I_GIT_COMMIT="" prior to build
s2i::build::get_version_vars
s2i::build::save_version_vars "${context}/sti-version-defs"

echo "++ Building release ${S2I_GIT_VERSION}"

# Create the input archive.
git archive --format=tar -o "${context}/archive.tar" "${S2I_GIT_COMMIT}"
tar -rf "${context}/archive.tar" -C "${context}" sti-version-defs
gzip -f "${context}/archive.tar"

# Perform the build and release in Docker.
cat "${context}/archive.tar.gz" | docker run -i --cidfile="${context}/cid" -e RELEASE_LDFLAGS="-w -s" openshift/sti-release
docker cp $(cat ${context}/cid):/go/src/github.com/openshift/source-to-image/_output/local/releases "${S2I_OUTPUT}"
echo "${S2I_GIT_COMMIT}" > "${S2I_LOCAL_RELEASEPATH}/.commit"

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
