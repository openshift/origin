#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

cd "${OS_ROOT}"

GENERATED_DOCS_ROOT="${OS_ROOT}/docs/generated"
_tmp="${OS_ROOT}/_tmp"
TMP_GENERATED_DOCS_ROOT="${_tmp}/generated"

mkdir -p "${_tmp}"
cp -a "${GENERATED_DOCS_ROOT}/" "${_tmp}"

"${OS_ROOT}/hack/update-generated-docs.sh"
echo "diffing ${GENERATED_DOCS_ROOT} against freshly generated docs"
ret=0
diff -Naupr "${GENERATED_DOCS_ROOT}" "${TMP_GENERATED_DOCS_ROOT}" || ret=$?
cp -a "${TMP_GENERATED_DOCS_ROOT}" "${GENERATED_DOCS_ROOT}/.."
rm -rf "${_tmp}"
if [[ $ret -eq 0 ]]
then
  echo "${GENERATED_DOCS_ROOT} up to date."
else
  echo "${GENERATED_DOCS_ROOT} is out of date. Please run hack/update-generated-docs.sh"
  exit 1
fi
