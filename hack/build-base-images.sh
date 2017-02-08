#!/bin/bash

# This script builds the base and release images for use by the release build and image builds.

STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

# determine the correct tag prefix
tag_prefix="${OS_IMAGE_PREFIX:-"openshift/origin"}"

# Build the base image without the default image args
OS_BUILD_IMAGE_ARGS="${OS_BUILD_IMAGE_BASE_ARGS-}" os::build::image "${OS_ROOT}/images/base" "${tag_prefix}-base"

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
