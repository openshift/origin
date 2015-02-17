#!/bin/bash

# This script builds all images locally except the base and release images,
# which are handled by hack/build-base-images.sh.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

# Get the latest Linux release
if [[ ! -d _output/local/releases ]]; then
  echo "No release has been built. Run hack/build-release.sh"
  exit 1
fi

# Extract the release achives to a staging area.
os::build::detect_local_release_tars "linux"

echo "Building images from release tars:"
echo " primary: $(basename ${OS_PRIMARY_RELEASE_TAR})"
echo " image:   $(basename ${OS_IMAGE_RELEASE_TAR})"

imagedir="_output/imagecontext"
rm -rf "${imagedir}"
mkdir -p "${imagedir}"
tar xzf "${OS_PRIMARY_RELEASE_TAR}" -C "${imagedir}"
tar xzf "${OS_IMAGE_RELEASE_TAR}" -C "${imagedir}"

# Copy primary binaries to the appropriate locations.
cp -f "${imagedir}/openshift" images/origin/bin
cp -f "${imagedir}/openshift" images/router/haproxy/bin

# Copy image binaries to the appropriate locations.
cp -f "${imagedir}/pod" images/pod/bin
cp -f "${imagedir}/hello-openshift" examples/hello-openshift/bin

# images that depend on scratch
echo "--- openshift/origin-pod ---"
docker build -t openshift/origin-pod:latest            images/pod

# images that depend on openshift/origin-base
echo "--- openshift/origin ---"
docker build -t openshift/origin:latest                images/origin
echo "--- openshift/origin-haproxy-router ---"
docker build -t openshift/origin-haproxy-router:latest images/router/haproxy
echo "--- openshift/hello-openshift ---"
docker build -t openshift/hello-openshift:latest       examples/hello-openshift

# images that depend on openshift/origin
echo "--- openshift/origin-deployer ---"
docker build -t openshift/origin-deployer:latest       images/deployer
echo "--- openshift/origin-docker-builder ---"
docker build -t openshift/origin-docker-builder:latest images/builder/docker/docker-builder
echo "--- openshift/origin-sti-builder ---"
docker build -t openshift/origin-sti-builder:latest    images/builder/docker/sti-builder
# unpublished images
echo "--- openshift/origin-custom-docker-builder ---"
docker build -t openshift/origin-custom-docker-builder:latest images/builder/docker/custom-docker-builder
echo "--- openshift/sti-image-builder ---"
docker build -t openshift/sti-image-builder:latest     images/builder/docker/sti-image-builder
