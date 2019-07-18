#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../../../k8s.io/code-generator)}

verify="${VERIFY:-}"

go install ./${CODEGEN_PKG}/cmd/deepcopy-gen

function codegen::join() { local IFS="$1"; shift; echo "$*"; }

# enumerate group versions
ALL_FQ_APIS=(
    github.com/openshift/template-service-broker/apis/config
    github.com/openshift/template-service-broker/apis/config/v1
    github.com/openshift/template-service-broker/apis/template
    github.com/openshift/template-service-broker/apis/template/v1
)

echo "Generating deepcopy funcs"
${GOPATH}/bin/deepcopy-gen --input-dirs $(codegen::join , "${ALL_FQ_APIS[@]}") -O zz_generated.deepcopy --bounding-dirs $(codegen::join , "${ALL_FQ_APIS[@]}") --go-header-file ${SCRIPT_ROOT}/hack/boilerplate.txt ${verify} "$@"
