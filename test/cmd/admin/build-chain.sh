#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
source "${OS_ROOT}/hack/lib/init.sh"
os::log::stacktrace::install
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/admin/build-chain"
# Test building a dependency tree
os::cmd::expect_success 'oc process -f examples/sample-app/application-template-stibuild.json -l build=sti | oc create -f -'
# Test both the type/name resource syntax and the fact that istag/origin-ruby-sample:latest is still
# not created but due to a buildConfig pointing to it, we get back its graph of deps.
os::cmd::expect_success_and_text 'oadm build-chain istag/origin-ruby-sample' 'istag/origin-ruby-sample:latest'
os::cmd::expect_success_and_text 'oadm build-chain ruby-22-centos7 -o dot' 'digraph "ruby-22-centos7:latest"'
os::cmd::expect_success 'oc delete all -l build=sti'
echo "ex build-chain: ok"
os::test::junit::declare_suite_end
