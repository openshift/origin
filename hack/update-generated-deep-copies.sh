#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../../../k8s.io/code-generator)}
verify="${VERIFY:-}"

go install ./${CODEGEN_PKG}/cmd/deepcopy-gen

function codegen::join() { local IFS="$1"; shift; echo "$*"; }

# enumerate group versions
ALL_FQ_APIS=(
    github.com/openshift/origin/pkg/build/controller/build/apis/defaults
    github.com/openshift/origin/pkg/build/controller/build/apis/defaults/v1
    github.com/openshift/origin/pkg/build/controller/build/apis/overrides
    github.com/openshift/origin/pkg/build/controller/build/apis/overrides/v1
    github.com/openshift/origin/pkg/build/controller/build/pluginconfig/testing
    github.com/openshift/origin/pkg/cmd/server/apis/config
    github.com/openshift/origin/pkg/cmd/server/apis/config/v1
    github.com/openshift/origin/pkg/cmd/server/apis/config/v1/testing
    github.com/openshift/origin/pkg/image/admission/apis/imagepolicy
    github.com/openshift/origin/pkg/image/admission/apis/imagepolicy/v1
    github.com/openshift/origin/pkg/ingress/admission/apis/ingressadmission
    github.com/openshift/origin/pkg/ingress/admission/apis/ingressadmission/v1
    github.com/openshift/origin/pkg/project/admission/lifecycle/testing
    github.com/openshift/origin/pkg/project/admission/apis/requestlimit
    github.com/openshift/origin/pkg/project/admission/apis/requestlimit/v1
    github.com/openshift/origin/pkg/quota/admission/apis/clusterresourceoverride
    github.com/openshift/origin/pkg/quota/admission/apis/clusterresourceoverride/v1
    github.com/openshift/origin/pkg/quota/admission/apis/runonceduration
    github.com/openshift/origin/pkg/quota/admission/apis/runonceduration/v1
    github.com/openshift/origin/pkg/router/f5/testing
    github.com/openshift/origin/pkg/scheduler/admission/apis/podnodeconstraints
    github.com/openshift/origin/pkg/scheduler/admission/apis/podnodeconstraints/v1
    github.com/openshift/origin/pkg/template/servicebroker/apis/config
    github.com/openshift/origin/pkg/template/servicebroker/apis/config/v1
    github.com/openshift/origin/pkg/util/testing
    github.com/openshift/origin/test/integration/testing
    github.com/openshift/origin/pkg/apps/apis/apps
    github.com/openshift/origin/pkg/authorization/apis/authorization
    github.com/openshift/origin/pkg/build/apis/build
    github.com/openshift/origin/pkg/image/apis/image
    github.com/openshift/origin/pkg/network/apis/network
    github.com/openshift/origin/pkg/oauth/apis/oauth
    github.com/openshift/origin/pkg/project/apis/project
    github.com/openshift/origin/pkg/quota/apis/quota
    github.com/openshift/origin/pkg/route/apis/route
    github.com/openshift/origin/pkg/security/apis/security
    github.com/openshift/origin/pkg/template/apis/template
    github.com/openshift/origin/pkg/user/apis/user
)

echo "Generating deepcopy funcs"
${GOPATH}/bin/deepcopy-gen --input-dirs $(codegen::join , "${ALL_FQ_APIS[@]}") -O zz_generated.deepcopy --bounding-dirs $(codegen::join , "${ALL_FQ_APIS[@]}") --go-header-file ${SCRIPT_ROOT}/hack/boilerplate.txt ${verify} "$@"
