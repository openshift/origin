#!/bin/bash

# This script sets up a go workspace locally and builds all go components.

set -o errexit
set -o nounset
set -o pipefail

hackdir=$(CDPATH="" cd $(dirname $0); pwd)

# Set the environment variables required by the build.
. "${hackdir}/config-go.sh"

# Go to the top of the tree.
cd "${OS_REPO_ROOT}"

# Fetch the version.
version=$(gitcommit)
kube_version=$(go run ${hackdir}/version.go ${hackdir}/../Godeps/Godeps.json github.com/GoogleCloudPlatform/kubernetes/pkg/api)

if [[ $# == 0 ]]; then
  # Update $@ with the default list of targets to build.
  set -- cmd/openshift
fi

binaries=()
for arg; do
  binaries+=("${OS_GO_PACKAGE}/${arg}")
done

build_tags=""
if [[ ! -z "$OS_BUILD_TAGS" ]]; then
  build_tags="-tags \"$OS_BUILD_TAGS\""
fi


go install $build_tags -ldflags "-X github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitCommit '${kube_version}' -X github.com/openshift/origin/pkg/version.commitFromGit '${version}'" "${binaries[@]}"
