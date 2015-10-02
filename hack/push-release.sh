#!/bin/bash

# This script pushes all of the built images to a registry.
#
# Set OS_PUSH_BASE_IMAGES=true to push base images
# Set OS_PUSH_BASE_REGISTRY to prefix the destination images
#

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

# Allow a release to be repushed with a tag
tag="${OS_PUSH_TAG:-}"
if [[ -n "${tag}" ]]; then
  tag=":${tag}"
else
  tag=":latest"
fi

# Source tag
source_tag="${OS_TAG:-}"
if [[ -z "${source_tag}" ]]; then
  source_tag="latest"
  file="${OS_ROOT}/_output/local/releases/.commit"
  if [[ -e ${file} ]]; then
    source_tag="$(cat $file)"
  fi
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
  openshift/origin-docker-registry
  openshift/origin-keepalived-ipfailover
  openshift/origin-sti-builder
  openshift/origin-haproxy-router
  openshift/origin-f5-router
  openshift/hello-openshift
)

PUSH_OPTS=""
if docker push --help | grep -q force; then
  PUSH_OPTS="--force"
fi

# Push the base images to a registry
if [[ "${tag}" == ":latest" ]]; then
  if [[ "${OS_PUSH_BASE_IMAGES-}" != "" ]]; then
    for image in "${base_images[@]}"; do
      if [[ "${OS_PUSH_BASE_REGISTRY-}" != "" ]]; then
        docker tag -f "${image}:${source_tag}" "${OS_PUSH_BASE_REGISTRY}${image}${tag}"
      fi
      docker push ${PUSH_OPTS} "${OS_PUSH_BASE_REGISTRY-}${image}${tag}"
    done
  fi
fi

# Pull latest in preparation for tagging
if [[ "${tag}" != ":latest" ]]; then
  if [[ -z "${OS_PUSH_LOCAL-}" ]]; then
    set -e
    for image in "${images[@]}"; do
      docker pull "${OS_PUSH_BASE_REGISTRY-}${image}:${source_tag}"
    done
    set +e
  else
    echo "WARNING: Pushing local :${source_tag} images to ${OS_PUSH_BASE_REGISTRY-}*${tag}"
    echo "  CTRL+C to cancel, or any other key to continue"
    read
  fi
fi

if [[ "${OS_PUSH_BASE_REGISTRY-}" != "" || "${tag}" != "" ]]; then
  set -e
  for image in "${images[@]}"; do
    docker tag -f "${image}:${source_tag}" "${OS_PUSH_BASE_REGISTRY-}${image}${tag}"
  done
  set +e
fi

for image in "${images[@]}"; do
  docker push ${PUSH_OPTS} "${OS_PUSH_BASE_REGISTRY-}${image}${tag}"
done

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
