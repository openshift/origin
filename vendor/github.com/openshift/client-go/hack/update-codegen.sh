#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../../../k8s.io/code-generator)}

verify="${VERIFY:-}"

for group in apps authorization build image network oauth project quota route security template user; do
  ${CODEGEN_PKG}/generate-groups.sh "client,lister,informer" \
    github.com/openshift/client-go/${group} \
    github.com/openshift/api \
    "${group}:v1" \
    --go-header-file ${SCRIPT_ROOT}/hack/boilerplate.txt \
    ${verify}
done
