#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete cm --all
  oc delete secret test
  exit 0
) &>/dev/null


os::test::junit::declare_suite_start "cmd/oc/set/data"

os::cmd::expect_success 'oc create configmap test'
tmpfile="$(mktemp)"
echo "c" > "${tmpfile}"

# test --local flag
os::cmd::expect_failure_and_text 'oc set data cm/test a=b --local' 'provide one or more resources by argument or filename'
# test --dry-run flag with -o formats
os::cmd::expect_success_and_text 'oc set data cm/test a=b --dry-run' 'test'
os::cmd::expect_success_and_text 'oc set data cm/test a=b --dry-run -o name' 'configmap/test'
os::cmd::expect_success_and_text 'oc set data cm/test a=b --dry-run' 'configmap/test data updated \(dry run\)'

os::cmd::expect_failure_and_text 'oc set data cm/test a=c a-' 'you may not remove and set the key'
os::cmd::expect_failure_and_text 'oc set data cm/test a=c --from-literal=a=b' 'cannot set key "a" in both argument and flag'

os::cmd::expect_success 'oc set data cm/test a=b'
os::cmd::expect_success_and_text "oc get cm/test -o jsonpath='{.data.a}'" 'b'
os::cmd::expect_success_and_text 'oc set data cm/test a=b' 'info: test was not changed'

os::cmd::expect_success 'oc set data cm/test a-'
os::cmd::expect_success_and_text "oc get cm/test -o jsonpath='{.data.a}'" ''
os::cmd::expect_success_and_text 'oc set data cm/test a-' 'info: test was not changed'

os::cmd::expect_success "oc set data cm/test --from-file=b=${tmpfile}"
os::cmd::expect_success_and_text "oc get cm/test -o jsonpath='{.data.b}'" 'c'
os::cmd::expect_success_and_text "oc set data cm/test --from-file=b=${tmpfile}" 'info: test was not changed'

rm -rf ${tmpfile}
mkdir -p ${tmpfile}
echo '1' > "${tmpfile}/a"
echo '2' > "${tmpfile}/b"
os::cmd::expect_success 'oc set data cm/test b-'
os::cmd::expect_success "oc set data cm/test --from-file=${tmpfile}"
os::cmd::expect_success_and_text "oc get cm/test -o jsonpath='{.data.a}'" '1'
os::cmd::expect_success_and_text "oc get cm/test -o jsonpath='{.data.b}'" '2'
os::cmd::expect_success_and_text "oc set data cm/test --from-file=${tmpfile}" "info: test was not changed"

os::cmd::expect_success 'oc create secret generic test'
os::cmd::expect_success 'oc set data secret/test a=b'
os::cmd::expect_success_and_text "oc get secret/test -o jsonpath='{.data.a}'" 'Yg=='
os::cmd::expect_success_and_text 'oc set data secret/test a=b' 'info: test was not changed'


echo "set-data: ok"
os::test::junit::declare_suite_end
