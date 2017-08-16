#!/bin/bash

# This script builds the base images for use by the release build and image builds.

STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

# determine the correct tag prefix
tag_prefix="${OS_IMAGE_PREFIX:-"openshift/origin"}"

result=0

# If OS_BUILD_ARCHES is not specified, default to the host architecture
build_arches="${OS_BUILD_ARCHES:-$(os::build::go_arch)}"

for image_name in "source" "base"; do
  if ! os::build::cross_images "${tag_prefix}-${image_name}" "${OS_ROOT}/images/${image_name}" "${build_arches}"; then
    result=1
    break
  fi
done

ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$result"
