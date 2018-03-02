#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/create"
# validate --dry-run outputs correct success message
os::cmd::expect_success_and_text 'oc create quota quota --dry-run' 'resourcequota "quota" created \(dry run\)'
os::cmd::expect_failure_and_text 'oc create -f - >> project_xiaocwan << __EOF__
apiVersion: v1
displayName: xiaocwan test
kind: Project
metadata:
  annotations:
    description: This is a test project of xiaocwan
    openshift.io/node-selector: env,qa
  labels:
    name: xiaocwan
  name: xiaocwan
__EOF__' 'The Project "xiaocwan" is invalid: nodeSelector: Invalid value: "env,qa": must be a valid label selector'

echo "oc create: ok"
os::test::junit::declare_suite_end