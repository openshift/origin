#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=/home/cloud-user/origin
source "${OS_ROOT}/hack/common.sh"

os::build::get_version_vars
# OS_GIT_VERSION is like 'v0.3.1-4-g2c853ed-dirty'
IMAGE_VERSION=`echo ${OS_GIT_VERSION} | cut -d '-' -f 1`

for i in ose-deployer ose-docker-builder ose-docker-registry ose-keepalived-ipfailover ose-haproxy-router ose-pod ose-sti-builder ose-sti-image-builder;
do
  docker save localhost:5000/openshift3/${i}:${IMAGE_VERSION} | gzip > ~/openshift3--${i}-${IMAGE_VERSION}.tar.gz
done

echo "Images for ${IMAGE_VERSION} saved to ~/openshift3--IMAGE-NAME-${IMAGE_VERSION}.tar.gz"

STI_BAU_VERSION="latest"
docker save localhost:5000/openshift3/sti-basicauthurl:${STI_BAU_VERSION} | gzip > ~/openshift3--sti-basicauthurl-${STI_BAU_VERSION}.tar.gz

echo "Image saved to ~/openshift3--sti-basicauthurl-${STI_BAU_VERSION}.tar.gz"
