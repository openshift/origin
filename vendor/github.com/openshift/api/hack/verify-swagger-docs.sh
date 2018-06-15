#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

SCRIPT_ROOT=$(dirname "${BASH_SOURCE}")/..
TMP_ROOT="${SCRIPT_ROOT}/_tmp"

cleanup() {
  rm -rf "${TMP_ROOT}"
}
trap "cleanup" EXIT SIGINT

cleanup

for gv in ${API_GROUP_VERSIONS}; do
  mkdir -p "${TMP_ROOT}/${gv}"
  cp -a "${SCRIPT_ROOT}/${gv}"/* "${TMP_ROOT}/${gv}"
done

"${SCRIPT_ROOT}/hack/update-swagger-docs.sh"
echo "Checking against freshly generated swagger..."
for gv in ${API_GROUP_VERSIONS}; do
  ret=0
  diff -Naupr "${SCRIPT_ROOT}/${gv}"/types_swagger_doc_generated.go "${TMP_ROOT}/${gv}"/types_swagger_doc_generated.go || ret=$?
  if [[ $ret -ne 0 ]]; then
    cp -a "${TMP_ROOT}"/* "${SCRIPT_ROOT}/"
    echo "Swagger is out of date. Please run hack/update-swagger-docs.sh"
    exit 1
  fi
done
echo "Swagger up to date."
