#!/bin/bash

# This script builds the base and release images for use by the release build and image builds.

STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

HOST_ARCH=$(os::build::host_arch)

DOCKERFILE="Dockerfile"

if [[ "${HOST_ARCH}" != "amd64" ]]; then
  DOCKERFILE+=".${HOST_ARCH}"
fi

# determine the correct tag prefix
tag_prefix="${OS_IMAGE_PREFIX:-"openshift/origin"}"

BASE_EXTRA_TAG=""
if [[ "${HOST_ARCH}" == "amd64" ]]; then
  BASE_EXTRA_TAG="${tag_prefix}-base"
fi

# Build the base image without the default image args
OS_BUILD_IMAGE_ARGS="${OS_BUILD_IMAGE_BASE_ARGS-}" os::build::image "${OS_ROOT}/images/base" "${tag_prefix}-base-${HOST_ARCH}" ${DOCKERFILE} ${BASE_EXTRA_TAG}

# Build release image only on ppc64le platform because automatic dockerbuild is not supported for ppc64le platform in hub.docker.com
if [[ "${HOST_ARCH}" == "ppc64le" ]]; then
  GOLANG_VERSION="1.8"
  OS_BUILD_IMAGE_ARGS="${OS_BUILD_IMAGE_BASE_ARGS-}" os::build::image "${OS_ROOT}/images/release/golang-${GOLANG_VERSION}" "${tag_prefix}-release-${HOST_ARCH}" ${DOCKERFILE} "${tag_prefix}-release-${HOST_ARCH}:golang-${GOLANG_VERSION}"
fi

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
