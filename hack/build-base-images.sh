#!/bin/bash

# This script builds the base and release images for use by the release build and image builds.

STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

# determine the correct tag prefix
tag_prefix="${OS_IMAGE_PREFIX:-"openshift/origin"}"

os::util::ensure::gopath_binary_exists imagebuilder

# Build the base image without the default image args
os::build::image "${tag_prefix}-source" "${OS_ROOT}/images/source"
os::build::image "${tag_prefix}-base"   "${OS_ROOT}/images/base"

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
