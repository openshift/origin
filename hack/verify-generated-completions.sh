#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

cd "${OS_ROOT}"

COMPLETION_ROOT="${OS_ROOT}/rel-eng/completions"
_tmp="${OS_ROOT}/_tmp"
TMP_COMPLETION_ROOT="${_tmp}/completions"

mkdir -p "${_tmp}"
cp -a "${COMPLETION_ROOT}/" "${_tmp}"

"${OS_ROOT}/hack/update-generated-completions.sh"
echo "diffing ${COMPLETION_ROOT} against freshly generated completions"
ret=0
diff -Naupr "${COMPLETION_ROOT}" "${TMP_COMPLETION_ROOT}" || ret=$?
cp -a "${TMP_COMPLETION_ROOT}" "${COMPLETION_ROOT}/.."
rm -rf "${_tmp}"
if [[ $ret -eq 0 ]]
then
  echo "${COMPLETION_ROOT} up to date."
else
  echo "${COMPLETION_ROOT} is out of date. Please run hack/update-generated-completions.sh"
  exit 1
fi
