#!/bin/bash

# This script generates release zips into _output/releases. It requires the openshift/origin-release
# image to be built prior to executing this command via hack/build-base-images.sh.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

context="${OS_ROOT}/_output/buildenv-context"

# clean existing output
rm -rf "${OS_ROOT}/_output/local/releases"
rm -rf "${OS_ROOT}/_output/local/go/bin"
rm -rf "${context}"
mkdir -p "${context}"
mkdir -p "${OS_ROOT}/_output/local"

# generate version definitions
os::build::get_version_vars
os::build::save_version_vars "${context}/os-version-defs"

# create the input archive
git archive --format=tar -o "${context}/archive.tar" HEAD
tar -rf "${context}/archive.tar" -C "${context}" os-version-defs
gzip -f "${context}/archive.tar"

# build in the clean environment
cat "${context}/archive.tar.gz" | docker run -i --cidfile="${context}/cid" openshift/origin-release
docker cp $(cat ${context}/cid):/go/src/github.com/openshift/origin/_output/local/releases "${OS_ROOT}/_output/local"

# copy the linux release back to the _output/go/bin dir
releases=$(find _output/local/releases/ -print | grep 'openshift-origin-.*-linux-' --color=never)
if [[ $(echo $releases | wc -l) -ne 1 ]]; then
  echo "There should be exactly one Linux release tar in _output/local/releases"
  exit 1
fi
bindir="_output/local/go/bin"
mkdir -p "${bindir}"
tar mxzf "${releases}" -C "${bindir}"
