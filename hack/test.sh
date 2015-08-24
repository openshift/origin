#!/bin/bash

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
os::build::setup_env

go test -v github.com/openshift/openshift-sdn/pkg/netutils
go test -v github.com/openshift/openshift-sdn/pkg/netutils/server
