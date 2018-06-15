#!/bin/bash

# This script builds release images for use by the release build.
#
# Set S2I_IMAGE_PUSH=true to push images to a registry
#

set -o errexit
set -o nounset
set -o pipefail

S2I_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${S2I_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${S2I_ROOT}"

# Build the images
docker build --tag openshift/sti-release "${S2I_ROOT}/images/release"
