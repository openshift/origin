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
releases=$(find _output/local/releases/ -print | grep 'openshift-origin-.*-linux-' --color=never)
if [[ $(echo $releases | wc -l) -ne 1 ]]; then
  echo "There must be exactly one Linux release tar in _output/local/releases"
  exit 1
fi
echo "Building images from ${releases}"

imagedir="_output/imagecontext"
rm -rf "${imagedir}"
mkdir -p "${imagedir}"
tar xzf "${releases}" -C "${imagedir}"

# copy build artifacts to the appropriate locations
cp -f "${imagedir}/openshift"        images/origin/bin
cp -f "${imagedir}/openshift-deploy" images/origin/bin
cp -f "${imagedir}/openshift-router" images/origin/bin
cp -f "${imagedir}/openshift-router" images/router/haproxy/bin
cp -f "${imagedir}/openshift-docker-build" images/builder/docker/docker-builder/bin
cp -f "${imagedir}/openshift-sti-build"    images/builder/docker/sti-builder/bin

# build hello-openshift binary
"${OS_ROOT}/hack/build-go.sh" examples/hello-openshift

# images that depend on openshift/origin-base
echo "--- openshift/origin ---"
docker build -t openshift/origin                images/origin
echo "--- openshift/origin-haproxy-router ---"
docker build -t openshift/origin-haproxy-router images/router/haproxy
echo "--- openshift/hello-openshift ---"
docker build -t openshift/hello-openshift       examples/hello-openshift

# images that depend on openshift/origin
echo "--- openshift/origin-deployer ---"
docker build -t openshift/origin-deployer       images/deploy/customimage
echo "--- openshift/origin-docker-builder ---"
docker build -t openshift/origin-docker-builder images/builder/docker/docker-builder
echo "--- openshift/origin-sti-builder ---"
docker build -t openshift/origin-sti-builder    images/builder/docker/sti-builder
