#!/bin/bash

set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/util.sh"
cd "${OS_ROOT}"

ensure_iptables_or_die


function cleanup()
{
	out=$?
	cleanup_openshift
	echo "[INFO] Exiting"
	exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

TMPDIR="${TMPDIR:-"/tmp"}"
BASETMPDIR="${TMPDIR}/openshift-extended-tests/config-compatibility"

# run the end-to-end using the old config from each release
V1_TMPDIR=${BASETMPDIR}/v1.0.0
sudo rm -rf "${V1_TMPDIR}"
mkdir -p "${V1_TMPDIR}"
set -e
BASETMPDIR=${V1_TMPDIR} test/old-start-configs/v1.0.0/test-end-to-end.sh
