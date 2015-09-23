#!/bin/bash

# This script builds the base and release images for use by the release build and image builds.
#
# Set OS_IMAGE_PUSH=true to push images to a registry
#

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# Go to the top of the tree.
cd "${OS_ROOT}"

# Build the images
docker build --tag openshift/origin-base                   "${OS_ROOT}/images/base"
docker build --tag openshift/origin-haproxy-router-base    "${OS_ROOT}/images/router/haproxy-base"
docker build --tag openshift/origin-release                "${OS_ROOT}/images/release"
