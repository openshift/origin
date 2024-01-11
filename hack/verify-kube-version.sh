#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

KUBE_GO_MOD_VERSION=$(grep "k8s.io/kubernetes v1" go.mod | sed 's/[\t]k8s.io\/kubernetes v//')

KUBE_DOCKER_VERSION=$(grep "kubernetes-tests=" images/tests/Dockerfile.rhel | sed 's/      io.openshift.build.versions="kubernetes-tests=//' | sed 's/" \\//')
if [[ "${KUBE_GO_MOD_VERSION}" != "${KUBE_DOCKER_VERSION}" ]]; then
  os::log::warning "kubernetes version (${KUBE_GO_MOD_VERSION}) and kubernetes-tests version (${KUBE_DOCKER_VERSION}) in images/tests/Dockerfile.rhel must be equal, please update Dockerfile"
  exit 1
fi
