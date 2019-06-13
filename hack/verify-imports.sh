#!/usr/bin/env bash

# This script verifies that package trees
# conform to our import restrictions
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    return_code=$?
    os::test::junit::generate_report
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

os::util::ensure::built_binary_exists 'import-verifier'

os::test::junit::declare_suite_start "verify/imports"
os::cmd::expect_success "import-verifier ${OS_ROOT}/hack/import-restrictions.json"

# quick and dirty check that nothing under vendored kubernetes imports something from origin
os::cmd::expect_failure "egrep -r '\"github.com/openshift/origin/[^\"]+\"$' vendor/k8s.io/kubernetes"

# quick and dirty check that nothing under origin staging imports from openshift/origin
os::cmd::expect_failure "go list -deps -test ./staging/src/github.com/openshift/... | grep 'openshift/origin/pkg'"

os::test::junit::declare_suite_end


function print_forbidden_imports () {
    set -o errexit # this was unset by ||
    local PACKAGE="$1"
    shift
    local RE=""
    local SEP=""
    for CLAUSE in "$@"; do
        RE+="${SEP}${CLAUSE}"
        SEP='\|'
    done
    local FORBIDDEN=$(
        go list -f $'{{with $package := .ImportPath}}{{range $.Imports}}{{$package}} imports {{.}}\n{{end}}{{end}}' ./vendor/github.com/openshift/${PACKAGE}/... |
        sed 's|^github.com/openshift/origin/vendor/||;s| github.com/openshift/origin/vendor/| |' |
        grep -v " github.com/openshift/${PACKAGE}" |
        grep -e "\( github.com/openshift/\| k8s.io/kubernetes\)" |
        grep -v "imports github.com/openshift/api" |
        grep -v "imports github.com/openshift/client-go" |
        grep -v "imports github.com/openshift/library-go" |
        grep -v -e "imports \(${RE}\)"
    )
    if [ -n "${FORBIDDEN}" ]; then
        echo "${PACKAGE} has a forbidden dependency:"
        echo
        echo "${FORBIDDEN}" | sed 's/^/  /'
        echo
        return 1
    fi
    local TEST_FORBIDDEN=$(
        go list -f $'{{with $package := .ImportPath}}{{range $.TestImports}}{{$package}} imports {{.}}\n{{end}}{{end}}' ./vendor/github.com/openshift/${PACKAGE}/... |
        sed 's|^github.com/openshift/origin/vendor/||;s| github.com/openshift/origin/vendor/| |' |
        grep -v " github.com/openshift/${PACKAGE}" |
        grep -e "\( github.com/openshift/\| k8s.io/kubernetes\)" |
        grep -v "imports github.com/openshift/api" |
        grep -v "imports github.com/openshift/client-go" |
        grep -v "imports github.com/openshift/library-go" |
        grep -v -e "imports \(${RE}\)"
    )
    if [ -n "${TEST_FORBIDDEN}" ]; then
        echo "${PACKAGE} has a forbidden dependency in test code:"
        echo
        echo "${TEST_FORBIDDEN}" | sed 's/^/  /'
        echo
        return 1
    fi
    return 0
}

# for some reason, if you specify nothing, then you never get an error.  Specify something, even if it never shows up
RC=0
print_forbidden_imports oauth-server k8s.io/kubernetes/pkg/apis || RC=1
print_forbidden_imports oc github.com/openshift/source-to-image k8s.io/kubernetes/pkg || RC=1
print_forbidden_imports openshift-apiserver k8s.io/kubernetes/pkg/apis || RC=1
print_forbidden_imports openshift-controller-manager k8s.io/kubernetes/cmd/controller-manager/app \
  k8s.io/kubernetes/pkg/api/legacyscheme \
  k8s.io/kubernetes/pkg/api/testing \
  k8s.io/kubernetes/pkg/apis \
  k8s.io/kubernetes/pkg/client/metrics/prometheus \
  k8s.io/kubernetes/pkg/controller \
  k8s.io/kubernetes/pkg/credentialprovider \
  k8s.io/kubernetes/pkg/kubectl/cmd/util \
  k8s.io/kubernetes/pkg/kubectl/util/templates \
  k8s.io/kubernetes/pkg/quota/v1 \
  k8s.io/kubernetes/pkg/registry/core/secret \
  k8s.io/kubernetes/pkg/registry/core/service || RC=1
print_forbidden_imports sdn k8s.io/kubernetes/cmd/kube-proxy/app \
  k8s.io/kubernetes/pkg/api/legacyscheme \
  k8s.io/kubernetes/pkg/apis \
  k8s.io/kubernetes/pkg/client/metrics/prometheus \
  k8s.io/kubernetes/pkg/kubectl/cmd/util \
  k8s.io/kubernetes/pkg/kubectl/util/templates \
  k8s.io/kubernetes/pkg/kubelet \
  k8s.io/kubernetes/pkg/proxy \
  k8s.io/kubernetes/pkg/registry/core/service/allocator \
  k8s.io/kubernetes/pkg/util || RC=1
print_forbidden_imports template-service-broker k8s.io/kubernetes/pkg/apis k8s.io/kubernetes/pkg/api k8s.io/kubernetes/pkg/kubectl k8s.io/kubernetes/pkg/controller || RC=1
if [ ${RC} != 0 ]; then
    exit ${RC}
fi

if grep -rq '// import "github.com/openshift/origin/' 'staging/'; then
	echo 'file has "// import "github.com/openshift/origin/"'
	exit 1
fi

exit 0
