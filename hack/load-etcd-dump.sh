#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

os::build::setup_env

export BASETMPDIR="/tmp/openshift/load-etcd-dump"
rm -rf ${BASETMPDIR} || true

go run tools/testdebug/load_etcd_dump.go $@
