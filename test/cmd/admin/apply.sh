#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/admin/apply"
workingdir=$(mktemp -d)
os::cmd::expect_success "oadm registry --credentials=${KUBECONFIG} -o yaml > ${workingdir}/oadm_registry.yaml"
os::util::sed "s/5000/6000/g" ${workingdir}/oadm_registry.yaml
os::cmd::expect_success "oc apply -f ${workingdir}/oadm_registry.yaml"
os::cmd::expect_success_and_text 'oc get dc/docker-registry -o yaml' '6000'
echo "apply: ok"
os::test::junit::declare_suite_end
