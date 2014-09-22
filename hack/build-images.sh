#!/bin/bash

# This script builds all images locally (requires Docker)

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

docker build -t openshift/docker-builder images/builder/docker/docker-builder
docker build -t openshift/sti-builder images/builder/docker/sti-builder
docker build -t openshift/hello-openshift examples/hello-openshift

images/deployer/kube-deploy/build.sh
docker build -t openshift/kube-deploy images/deployer/kube-deploy
rm -f images/deployer/kube-deploy/kube-deploy
