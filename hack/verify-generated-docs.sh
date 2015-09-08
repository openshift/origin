#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

cd "${OS_ROOT}"

echo "===== Verifying Generated Docs ====="

GENERATED_DOCS_ROOT_REL="docs/generated"
GENERATED_DOCS_ROOT="${OS_ROOT}/${GENERATED_DOCS_ROOT_REL}"
TMP_GENERATED_DOCS_ROOT_REL="_output/verify-generated-docs"
TMP_GENERATED_DOCS_ROOT="${OS_ROOT}/${TMP_GENERATED_DOCS_ROOT_REL}/${GENERATED_DOCS_ROOT_REL}"

echo "Generating fresh docs..."
if ! output=`${OS_ROOT}/hack/update-generated-docs.sh ${TMP_GENERATED_DOCS_ROOT_REL} 2>&1`
then
	echo "FAILURE: Generation of fresh docs failed:"
	echo "$output"
fi

echo "Diffing current docs against freshly generated docs"
ret=0
diff -Naupr "${GENERATED_DOCS_ROOT}" "${TMP_GENERATED_DOCS_ROOT}" || ret=$?
rm -rf "${TMP_GENERATED_DOCS_ROOT}"
if [[ $ret -eq 0 ]]
then
  echo "SUCCESS: Generated docs up to date."
else
  echo "FAILURE: Generated docs out of date. Please run hack/update-generated-docs.sh"
  exit 1
fi
