#!/bin/bash

# This script builds the base and release images for use by the release build and image builds.

STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::util::ensure::built_binary_exists 'oc'

function build() {
  eval "'${oc}' ex dockerbuild $2 $1 ${OS_BUILD_IMAGE_ARGS:-}"
}

# Build the images
build openshift/origin-base                   "${OS_ROOT}/images/base"
build openshift/origin-haproxy-router-base    "${OS_ROOT}/images/router/haproxy-base"
build openshift/origin-release                "${OS_ROOT}/images/release"

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
