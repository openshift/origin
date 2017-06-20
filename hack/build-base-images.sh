#!/bin/bash

# This script builds the base and release images for use by the release build and image builds.

STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

host_arch="$(os::util::host_arch)"
dockerfile_append=
golang_versions=

# Special case alt arches
if [[ "${host_arch}" == "amd64" ]]; then
  golang_versions="1.7 1.8"
elif [[ "${host_arch}" == "ppc64le" ]]; then
  dockerfile_append=".altarch"
  golang_versions=1.8
elif [[ "${host_arch}" == "arm64" ]]; then
  dockerfile_append=".altarch"
  golang_versions=1.8
else
  echo "Architecture: ${host_arch} not supported."
  exit 1
fi

# determine the correct tag prefix
tag_prefix="${OS_IMAGE_PREFIX:-"openshift/origin"}"

os::util::ensure::gopath_binary_exists imagebuilder

# Build the base image without the default image args
ORIG_BUILD_IMAGE_ARGS=${OS_BUILD_IMAGE_ARGS:-}

image_dir="${OS_ROOT}/images/source"
OS_BUILD_IMAGE_ARGS="-f ${image_dir}/Dockerfile --from $(os::util::centos_image) ${ORIG_BUILD_IMAGE_ARGS}"
os::build::image "${tag_prefix}-source" "${image_dir}"

image_dir="${OS_ROOT}/images/base"
OS_BUILD_IMAGE_ARGS="-f ${image_dir}/Dockerfile --from ${tag_prefix}-source ${ORIG_BUILD_IMAGE_ARGS}"
os::build::image "${tag_prefix}-base" "${image_dir}"

for go_ver in ${golang_versions}; do
  image_dir="${OS_ROOT}/images/release/golang-${go_ver}"
  OS_BUILD_IMAGE_ARGS="-f ${image_dir}/Dockerfile${dockerfile_append} --from ${tag_prefix}-base ${ORIG_BUILD_IMAGE_ARGS}"
  os::build::image "${tag_prefix}-release:golang-${go_ver}" "${image_dir}"
done

OS_BUILD_IMAGE_ARGS=${ORIG_BUILD_IMAGE_ARGS}

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
