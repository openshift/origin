#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

os::build::get_version_vars
# OS_GIT_VERSION is like 'v0.3.1-4-g2c853ed-dirty'
IMAGE_VERSION=`echo ${OS_GIT_VERSION} | cut -d '-' -f 1`

docker build --no-cache=true -t openshift3_beta/ose-base ./base && \
docker build --no-cache=true -t openshift3_beta/ose ./ose/ && \
docker build --no-cache=true -t openshift3_beta/ose-haproxy-router-base ./router/haproxy-base/ && \
docker build --no-cache=true -t openshift3_beta/ose-haproxy-router ./router/haproxy/ && \
docker build --no-cache=true -t openshift3_beta/ose-deployer ./deployer/ && \
docker build --no-cache=true -t openshift3_beta/ose-docker-builder ./builder/docker/docker-builder/ && \
docker build --no-cache=true -t openshift3_beta/ose-sti-builder ./builder/docker/sti-builder/ && \
docker build --no-cache=true -t openshift3_beta/ose-sti-image-builder ./builder/docker/sti-image-builder/ && \
docker build --no-cache=true -t openshift3_beta/ose-pod ./pod/

docker tag openshift3_beta/ose-docker-builder localhost:5000/openshift3_beta/ose-docker-builder
docker tag openshift3_beta/ose-docker-builder localhost:5000/openshift3_beta/ose-docker-builder:${IMAGE_VERSION}

docker tag openshift3_beta/ose-sti-builder localhost:5000/openshift3_beta/ose-sti-builder
docker tag openshift3_beta/ose-sti-builder localhost:5000/openshift3_beta/ose-sti-builder:${IMAGE_VERSION}

docker tag openshift3_beta/ose-sti-image-builder localhost:5000/openshift3_beta/ose-sti-image-builder
docker tag openshift3_beta/ose-sti-image-builder localhost:5000/openshift3_beta/ose-sti-image-builder:${IMAGE_VERSION}

docker tag openshift3_beta/ose-deployer localhost:5000/openshift3_beta/ose-deployer
docker tag openshift3_beta/ose-deployer localhost:5000/openshift3_beta/ose-deployer:${IMAGE_VERSION}

docker tag openshift3_beta/ose-haproxy-router localhost:5000/openshift3_beta/ose-haproxy-router
docker tag openshift3_beta/ose-haproxy-router localhost:5000/openshift3_beta/ose-haproxy-router:${IMAGE_VERSION}

docker tag openshift3_beta/ose-pod localhost:5000/openshift3_beta/ose-pod
docker tag openshift3_beta/ose-pod localhost:5000/openshift3_beta/ose-pod:${IMAGE_VERSION}



docker push localhost:5000/openshift3_beta/ose-docker-builder &&
docker push localhost:5000/openshift3_beta/ose-docker-builder:${IMAGE_VERSION} &&
docker push localhost:5000/openshift3_beta/ose-sti-builder &&
docker push localhost:5000/openshift3_beta/ose-sti-builder:${IMAGE_VERSION} &&
docker push localhost:5000/openshift3_beta/ose-sti-image-builder &&
docker push localhost:5000/openshift3_beta/ose-sti-image-builder:${IMAGE_VERSION} &&
docker push localhost:5000/openshift3_beta/ose-deployer &&
docker push localhost:5000/openshift3_beta/ose-deployer:${IMAGE_VERSION} &&
docker push localhost:5000/openshift3_beta/ose-haproxy-router &&
docker push localhost:5000/openshift3_beta/ose-haproxy-router:${IMAGE_VERSION} &&
docker push localhost:5000/openshift3_beta/ose-pod &&
docker push localhost:5000/openshift3_beta/ose-pod:${IMAGE_VERSION}
