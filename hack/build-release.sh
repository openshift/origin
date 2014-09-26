#!/bin/bash

# This script generates a release script in _output/releases

set -o errexit
set -o nounset
set -o pipefail

hackdir=$(CDPATH="" cd $(dirname $0); pwd)


# Set the environment variables required by the build.
. "${hackdir}/config-go.sh"

# Go to the top of the tree.
cd "${OS_REPO_ROOT}"

# Build clean
make clean
hack/build-go.sh

# Fetch the version.
version=$(gitcommit)

# Copy built contents to the release directory
release="_output/release"
rm -rf "${release}"
mkdir -p "${release}"
cp _output/go/bin/openshift "${release}"

releases="_output/releases"
mkdir -p "${releases}"
release_file="${releases}/openshift-origin-linux64-${version}.tar.gz"

tar cvzf "${release_file}" -C "${release}" .

echo "Built to ${release_file}"