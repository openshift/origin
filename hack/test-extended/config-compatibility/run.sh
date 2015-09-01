#!/bin/bash

set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/util.sh"
cd "${OS_ROOT}"

test_privileges

trap "exit" INT TERM
trap "cleanup_extended" EXIT

TMPDIR="${TMPDIR:-"/tmp"}"
BASETMPDIR="${TMPDIR}/openshift-extended-test/config-compatibility"

# run the end-to-end using the old config from each release
V1_TMPDIR=${BASETMPDIR}/v1.0.0
sudo rm -rf "${V1_TMPDIR}"
mkdir -p "${V1_TMPDIR}"
set -e
BASETMPDIR=${V1_TMPDIR} test/old-start-configs/v1.0.0/test-end-to-end.sh