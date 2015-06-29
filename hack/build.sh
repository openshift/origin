#!/bin/bash

set -e

OSDN_ROOT=$(
  unset CDPATH
  osdn_root=$(dirname "${BASH_SOURCE}")/..
  cd "${osdn_root}"
  pwd
)

OS_OUTPUT="${OSDN_ROOT}/_output/local"
readonly OSDN_GO_PACKAGE=github.com/openshift/openshift-sdn
readonly OSDN_GOPATH="${OSDN_ROOT}/_output/local/go"

setup_env() {
  if [[ -z "$(which go)" ]]; then
    echo "Can't find 'go' in PATH, please fix and retry."
    exit 2
  fi

  local go_pkg_dir="${OSDN_GOPATH}/src/${OSDN_GO_PACKAGE}"
  local go_pkg_basedir=$(dirname "${go_pkg_dir}")
  mkdir -p "${go_pkg_basedir}"
  mkdir -p "${OSDN_GOPATH}/bin"
  rm -f "${go_pkg_dir}"

  # TODO: This symlink should be relative.
  ln -s "${OSDN_ROOT}" "${go_pkg_dir}"

  GOPATH=${OSDN_GOPATH}:${OSDN_ROOT}/Godeps/_workspace
  export GOPATH
}

setup_env
go install ${OSDN_GO_PACKAGE}
cp -f ovssubnet/controller/lbr/bin/openshift-sdn-simple-setup-node.sh ${OSDN_GOPATH}/bin
cp -f ovssubnet/controller/kube/bin/openshift-ovs-subnet ${OSDN_GOPATH}/bin
cp -f ovssubnet/controller/kube/bin/openshift-sdn-kube-subnet-setup.sh ${OSDN_GOPATH}/bin
