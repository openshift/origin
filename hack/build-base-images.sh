#!/bin/bash

# This script builds the base and release images for use by the release build and image builds.

STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

# determine the correct tag prefix
tag_prefix="${OS_IMAGE_PREFIX:-"openshift/origin"}"

os::util::ensure::gopath_binary_exists imagebuilder

# image builds require RPMs to have been built
if [[ ! -d "${OS_LOCAL_RPMPATH}" ]]; then
	relative_releasepath="$( os::util::repository_relative_path "${OS_LOCAL_RELEASEPATH}" )"
	relative_binpath="$( os::util::repository_relative_path "${OS_OUTPUT_BINPATH}" )"
	os::log::fatal "No release RPMs have been built! RPMs are necessary to build container images.
Build them with:
  $ OS_BUILD_ENV_PRESERVE=${relative_binpath}:${relative_releasepath} hack/env make build-rpms-redistributable"
fi

OS_BUILD_IMAGE_BASE_ARGS="${OS_BUILD_IMAGE_BASE_ARGS:-} -mount ${OS_LOCAL_RPMPATH}/:/srv/origin-local-release/"

# Build the base image without the default image args
OS_BUILD_IMAGE_ARGS="${OS_BUILD_IMAGE_BASE_ARGS-}" os::build::image "${OS_ROOT}/images/source" "${tag_prefix}-source"
OS_BUILD_IMAGE_ARGS="${OS_BUILD_IMAGE_BASE_ARGS-}" os::build::image "${OS_ROOT}/images/base" "${tag_prefix}-base"

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
