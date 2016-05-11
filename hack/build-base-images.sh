#!/bin/bash

# This script builds the base and release images for use by the release build and image builds.
#
# Set OS_IMAGE_PUSH=true to push images to a registry
#

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# Go to the top of the tree.
cd "${OS_ROOT}"

oc="$(os::build::find-binary oc ${OS_ROOT})"
if [[ -z "${oc}" ]]; then
  "${OS_ROOT}/hack/build-go.sh" cmd/oc
  oc="$(os::build::find-binary oc ${OS_ROOT})"
fi

function build() {
  "${oc}" ex dockerbuild $2 $1
}

# Build the images
build openshift/origin-base                   "${OS_ROOT}/images/base"
build openshift/origin-haproxy-router-base    "${OS_ROOT}/images/router/haproxy-base"
build openshift/origin-release                "${OS_ROOT}/images/release"

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
