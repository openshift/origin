#!/bin/bash

# This script pushes all of the built images to a registry.
#
# Set OS_PUSH_BASE_IMAGES=true to push base images
# Set OS_PUSH_BASE_REGISTRY to prefix the destination images
#

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

# Push the base images to a registry
if [[ "${OS_PUSH_BASE_IMAGES-}" != "" ]]; then
  base_images=(
    openshift/origin-base
    openshift/origin-release
  )
  for image in "${base_images[@]}"; do
    if [[ "${OS_PUSH_BASE_REGISTRY-}" != "" ]]; then
      docker tag "${image}" "${OS_PUSH_BASE_REGISTRY}${image}"
    fi
    docker push "${OS_PUSH_BASE_REGISTRY-}${image}"
  done
fi

# Push the regular images to a registry
images=(
  openshift/origin
  openshift/origin-deployer
  openshift/origin-docker-builder
  openshift/origin-sti-builder
  openshift/origin-haproxy-router
  openshift/hello-openshift
)
for image in "${images[@]}"; do
  if [[ "${OS_PUSH_BASE_REGISTRY-}" != "" ]]; then
    docker tag "${image}" "${OS_PUSH_BASE_REGISTRY}${image}"
  fi
  docker push "${OS_PUSH_BASE_REGISTRY-}${image}"
done
