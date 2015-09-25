#!/bin/bash

OSDN_ROOT=$(readlink -f -- $(dirname "${BASH_SOURCE}")/..)

TEST_BASE_DIR=_output/src/github.com/openshift
# symlink $TEST_BASE_DIR/openshift-sdn to source dir
if [ ! -L "$TEST_BASE_DIR/openshift-sdn" ]; then
  mkdir -p $TEST_BASE_DIR || true
  pushd $TEST_BASE_DIR
  ln -s ../../../../ openshift-sdn || true
  popd
fi

GOPATH=${OSDN_ROOT}/_output:${OSDN_ROOT}/Godeps/_workspace
export GOPATH

go test -v github.com/openshift/openshift-sdn/pkg/netutils
go test -v github.com/openshift/openshift-sdn/pkg/netutils/server
