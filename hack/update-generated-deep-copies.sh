#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../../../k8s.io/code-generator)}
verify="${VERIFY:-}"

go install ./${CODEGEN_PKG}/cmd/deepcopy-gen

function codegen::join() { local IFS="$1"; shift; echo "$*"; }

# enumerate group versions
ALL_FQ_APIS=(
    github.com/openshift/origin/vendor/k8s.io/kubernetes/openshift-kube-apiserver/admission/autoscaling/apis/clusterresourceoverride
    github.com/openshift/origin/vendor/k8s.io/kubernetes/openshift-kube-apiserver/admission/autoscaling/apis/clusterresourceoverride/v1
    github.com/openshift/origin/vendor/k8s.io/kubernetes/openshift-kube-apiserver/admission/autoscaling/apis/runonceduration
    github.com/openshift/origin/vendor/k8s.io/kubernetes/openshift-kube-apiserver/admission/autoscaling/apis/runonceduration/v1
    github.com/openshift/origin/vendor/k8s.io/kubernetes/openshift-kube-apiserver/admission/network/apis/externalipranger
    github.com/openshift/origin/vendor/k8s.io/kubernetes/openshift-kube-apiserver/admission/network/apis/externalipranger/v1
    github.com/openshift/origin/vendor/k8s.io/kubernetes/openshift-kube-apiserver/admission/network/apis/restrictedendpoints
    github.com/openshift/origin/vendor/k8s.io/kubernetes/openshift-kube-apiserver/admission/network/apis/restrictedendpoints/v1
    github.com/openshift/origin/vendor/k8s.io/kubernetes/openshift-kube-apiserver/admission/route/apis/ingressadmission
    github.com/openshift/origin/vendor/k8s.io/kubernetes/openshift-kube-apiserver/admission/route/apis/ingressadmission/v1
    github.com/openshift/origin/vendor/k8s.io/kubernetes/openshift-kube-apiserver/admission/scheduler/apis/podnodeconstraints
    github.com/openshift/origin/vendor/k8s.io/kubernetes/openshift-kube-apiserver/admission/scheduler/apis/podnodeconstraints/v1
)

echo "Generating deepcopy funcs"
${GOPATH}/bin/deepcopy-gen --input-dirs $(codegen::join , "${ALL_FQ_APIS[@]}") -O zz_generated.deepcopy --bounding-dirs $(codegen::join , "${ALL_FQ_APIS[@]}") --go-header-file ${SCRIPT_ROOT}/vendor/k8s.io/kubernetes/hack/boilerplate/boilerplate.generatego.txt ${verify} "$@"
