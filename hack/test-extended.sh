#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

ensure_iptables_or_die


EXTENDED_BUCKET_STRING="${1:-all}"
EXTENDED_BUCKETS=(${EXTENDED_BUCKET_STRING//,/ })

if [ "${EXTENDED_BUCKET_STRING}" = "all" ]; then
	EXTENDED_BUCKETS=$(ls hack/test-extended)
fi


for BUCKET in ${EXTENDED_BUCKETS[@]}; do
	if [ -z `find hack/test-extended -type d -name ${BUCKET}` ]; then
	    echo "[ERROR] Extended test bucket ${BUCKET} not found"
	    exit 1
	fi
	
    echo "[INFO] Starting extended test ${BUCKET}"
	hack/test-extended/${BUCKET}/run.sh
done

echo "[INFO] Finished extended tests"
