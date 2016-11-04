#!/bin/bash

# This script pushes all of the built images to a registry.
#
# Set OS_PUSH_BASE_IMAGES=true to push base images
# Set OS_PUSH_BASE_REGISTRY to prefix the destination images
#
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

# Allow a release to be repushed with a tag
tag="${OS_PUSH_TAG:-}"
if [[ -n "${tag}" ]]; then
  if [[ "${tag}" == "HEAD" ]]; then
    if [[ "$( git tag --points-at HEAD | wc -l )" -ne 1 ]]; then
      echo "error: There must be exactly one tag pointing to HEAD to use OS_PUSH_TAG=HEAD"
      exit 1
    fi
    tag=":$( git tag --points-at HEAD)"
  else
    tag=":${tag}"
  fi
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
  openshift/origin-egress-router
  openshift/origin-recycler
  openshift/origin-gitserver
  openshift/hello-openshift
  openshift/openvswitch
  openshift/node
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
        docker tag "${image}:${source_tag}" "${OS_PUSH_BASE_REGISTRY}${image}${tag}"
      fi
      docker push ${PUSH_OPTS} "${OS_PUSH_BASE_REGISTRY-}${image}${tag}"
    done
  fi
fi

# Pull latest in preparation for tagging
if [[ "${tag}" != ":latest" ]]; then
  if [[ -z "${OS_PUSH_LOCAL-}" ]]; then
    for image in "${images[@]}"; do
      docker pull "${OS_PUSH_BASE_REGISTRY-}${image}:${source_tag}"
    done
  else
    os::log::warn "Pushing local :${source_tag} images to ${OS_PUSH_BASE_REGISTRY-}*${tag}"
    if [[ -z "${OS_PUSH_ALWAYS:-}" ]]; then
      echo "  CTRL+C to cancel, or any other key to continue"
      read
    fi
  fi
fi

if [[ "${OS_PUSH_BASE_REGISTRY-}" != "" || "${tag}" != "" ]]; then
  for image in "${images[@]}"; do
    docker tag "${image}:${source_tag}" "${OS_PUSH_BASE_REGISTRY-}${image}${tag}"
  done
fi

for image in "${images[@]}"; do
  docker push ${PUSH_OPTS} "${OS_PUSH_BASE_REGISTRY-}${image}${tag}"
done

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
