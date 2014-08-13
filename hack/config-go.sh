#!/bin/bash

# This script sets up a go workspace locally and builds all go components.
# You can 'source' this file if you want to set up GOPATH in your local shell.

if [ "$(which go)" == "" ]; then
	echo "Can't find 'go' in PATH, please fix and retry."
	echo "See http://golang.org/doc/install for installation instructions."
	exit 1
fi

# Travis continuous build uses a head go release that doesn't report
# a version number, so we skip this check on Travis.  Its unnecessary
# there anyway.
if [ "${TRAVIS}" != "true" ]; then
  GO_VERSION=($(go version))

  if [ ${GO_VERSION[2]} \< "go1.2" ]; then
    echo "Detected go version: ${GO_VERSION}."
    echo "OpenShift requires go version 1.2 or greater."
    echo "Please install Go version 1.2 or later"
    exit 1
  fi
fi

pushd $(dirname "${BASH_SOURCE}")/.. >/dev/null
OS_REPO_ROOT="${PWD}"
OS_TARGET="${OS_REPO_ROOT}/output/go"
popd >/dev/null

mkdir -p "${OS_TARGET}"

OLD_GOPATH="${GOPATH}"
export GOPATH="${OS_TARGET}"

OS_GO_PACKAGE="github.com/openshift/origin"
OS_GO_PACKAGE_DIR="${GOPATH}/src/${OS_GO_PACKAGE}"

ETCD_GO_PACKAGE="github.com/coreos/etcd"
ETCD_GO_PACKAGE_DIR="${GOPATH}/src/${ETCD_GO_PACKAGE}"
ETCD_EXISTING=$(GOPATH=${OLD_GOPATH} go list -f '{{.Dir}}' github.com/coreos/etcd)
if [ $? -ne 0 ]; then
  echo "You must go get ${ETCD_GO_PACKAGE}"
fi

(
  PACKAGE_BASE=$(dirname "${OS_GO_PACKAGE_DIR}")
  if [ ! -d "${PACKAGE_BASE}" ]; then
    mkdir -p "${PACKAGE_BASE}"
  fi
  rm "${OS_GO_PACKAGE_DIR}" >/dev/null 2>&1 || true
  ln -s "${OS_REPO_ROOT}" "${OS_GO_PACKAGE_DIR}"

  PACKAGE_BASE=$(dirname "${ETCD_GO_PACKAGE_DIR}")
  if [ ! -d "${PACKAGE_BASE}" ]; then
    mkdir -p "${PACKAGE_BASE}"
  fi
  rm "${ETCD_GO_PACKAGE_DIR}" >/dev/null 2>&1 || true
  ln -s "${ETCD_EXISTING}" "${ETCD_GO_PACKAGE_DIR}"


  if [[ "$OS_KUBE_PATH" != "" ]]; then
    echo "Using Kubernetes from source $OS_KUBE_PATH"
    OS_GO_KUBE_PACKAGE_DIR="${OS_TARGET}/src/github.com/GoogleCloudPlatform/kubernetes"
    KUBE_PACKAGE_BASE=$(dirname "${OS_GO_KUBE_PACKAGE_DIR}")
    if [ ! -d "${KUBE_PACKAGE_BASE}" ]; then
      mkdir -p "${KUBE_PACKAGE_BASE}"
    fi
    rm "${OS_GO_KUBE_PACKAGE_DIR}" >/dev/null 2>&1 || true
    ln -s "${OS_KUBE_PATH}" "${OS_GO_KUBE_PACKAGE_DIR}"
  fi
)
export GOPATH="${OS_TARGET}:${OS_REPO_ROOT}/third_party/src/github.com/GoogleCloudPlatform/kubernetes/third_party:${OS_REPO_ROOT}/third_party"
