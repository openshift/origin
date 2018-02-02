#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../../../k8s.io/code-generator)}

verify="${VERIFY:-}"

${CODEGEN_PKG}/generate-groups.sh "deepcopy" \
  github.com/openshift/api/generated \
  github.com/openshift/api \
  "apps:v1 authorization:v1 build:v1 image:v1,docker10,dockerpre012 network:v1 oauth:v1 project:v1 quota:v1 route:v1 security:v1 template:v1 user:v1 webconsole:v1" \
  --go-header-file ${SCRIPT_ROOT}/hack/empty.txt \
  ${verify}
