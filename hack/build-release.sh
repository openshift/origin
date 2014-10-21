#!/bin/bash

# This script generates release zips into _output/releases

set -o errexit
set -o nounset
set -o pipefail

hackdir=$(CDPATH="" cd $(dirname $0); pwd)

# Set the environment variables required by the build.
. "${hackdir}/config-go.sh"

# Go to the top of the tree.
cd "${OS_REPO_ROOT}"

context="${OS_REPO_ROOT}/_output/buildenv-context"

# clean existing output
rm -rf "${OS_REPO_ROOT}/_output/releases"
rm -rf "${context}"
mkdir -p "${context}"

# generate version definitions
echo "export OS_VERSION=0.1"                            > "${context}/os-version-defs"
echo "export OS_GITCOMMIT=\"$(os::build::gitcommit)\"" >> "${context}/os-version-defs"
echo "export OS_LD_FLAGS=\"$(os::build::ldflags)\""    >> "${context}/os-version-defs"

# create the input archive
git archive --format=tar -o "${context}/archive.tar" HEAD
tar -rf "${context}/archive.tar" -C "${context}" os-version-defs
gzip -f "${context}/archive.tar"

# build in the clean environment
docker build --tag openshift-origin-buildenv "${OS_REPO_ROOT}/hack/buildenv"
cat "${context}/archive.tar.gz" | docker run -i --cidfile="${context}/cid" openshift-origin-buildenv
docker cp $(cat ${context}/cid):/go/src/github.com/openshift/origin/_output/releases "${OS_REPO_ROOT}/_output"
