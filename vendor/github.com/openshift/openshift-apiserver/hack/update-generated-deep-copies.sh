#!/usr/bin/env bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../../../k8s.io/code-generator)}

verify="${VERIFY:-}"

go install ./${CODEGEN_PKG}/cmd/deepcopy-gen

function codegen::join() { local IFS="$1"; shift; echo "$*"; }

# enumerate group versions
ALL_FQ_APIS=(
    github.com/openshift/openshift-apiserver/pkg/project/apiserver/admission/apis/requestlimit
    github.com/openshift/openshift-apiserver/pkg/project/apiserver/admission/apis/requestlimit/v1
    github.com/openshift/openshift-apiserver/pkg/apps/apis/apps
    github.com/openshift/openshift-apiserver/pkg/authorization/apis/authorization
    github.com/openshift/openshift-apiserver/pkg/build/apis/build
    github.com/openshift/openshift-apiserver/pkg/image/apis/image
    github.com/openshift/openshift-apiserver/pkg/oauth/apis/oauth
    github.com/openshift/openshift-apiserver/pkg/project/apis/project
    github.com/openshift/openshift-apiserver/pkg/quota/apis/quota
    github.com/openshift/openshift-apiserver/pkg/route/apis/route
    github.com/openshift/openshift-apiserver/pkg/security/apis/security
    github.com/openshift/openshift-apiserver/pkg/template/apis/template
    github.com/openshift/openshift-apiserver/pkg/user/apis/user
)

echo "Generating deepcopy funcs"
${GOPATH}/bin/deepcopy-gen --input-dirs $(codegen::join , "${ALL_FQ_APIS[@]}") -O zz_generated.deepcopy --bounding-dirs $(codegen::join , "${ALL_FQ_APIS[@]}") --go-header-file ${SCRIPT_ROOT}/hack/boilerplate.txt ${verify} "$@"
