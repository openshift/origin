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

# Allow a release to be repushed with a tag
tag="${OS_PUSH_TAG:-}"
if [[ -n "${tag}" ]]; then
  tag=":${tag}"
fi

base_images=(
  openshift/origin-base
  openshift/origin-release
)
images=(
  openshift/origin
  openshift/origin-pod
  openshift/origin-deployer
  openshift/origin-docker-builder
  openshift/origin-sti-builder
  openshift/origin-haproxy-router
  openshift/hello-openshift
)

if [[ -z "${tag}" ]]; then
  # Push the base images to a registry
  if [[ "${OS_PUSH_BASE_IMAGES-}" != "" ]]; then
    for image in "${base_images[@]}"; do
      if [[ "${OS_PUSH_BASE_REGISTRY-}" != "" ]]; then
        docker tag "${image}" "${OS_PUSH_BASE_REGISTRY}${image}"
      fi
      docker push "${OS_PUSH_BASE_REGISTRY-}${image}"
    done
  fi
fi

# Pull latest in preparation for tagging
if [[ -n "${tag}" ]]; then
  set -e
  for image in "${images[@]}"; do
    docker pull "${OS_PUSH_BASE_REGISTRY-}${image}"
  done
  set +e
fi

if [[ "${OS_PUSH_BASE_REGISTRY-}" != "" || "${tag}" != "" ]]; then
  set -e
  for image in "${images[@]}"; do
    docker tag "${image}" "${OS_PUSH_BASE_REGISTRY-}${image}${tag}"
  done
  set +e
fi

for image in "${images[@]}"; do
  docker push "${OS_PUSH_BASE_REGISTRY-}${image}${tag}"
done
