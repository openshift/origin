#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,is,pods --all

  exit 0
) &> /dev/null

project="$( oc project -q )"

os::test::junit::declare_suite_start "cmd/image-lookup"
# This test validates image lookup resolution

# Verify image resolution on default resource types
os::cmd::expect_success          "oc import-image --confirm --from=nginx:latest nginx:latest"
os::cmd::expect_success_and_text "oc set image-lookup is/nginx" "updated"
# Image lookup works for pods
os::cmd::expect_success          "oc run --generator=run-pod/v1 --restart=Never --image=nginx:latest nginx"
os::cmd::expect_success_and_text "oc get pod/nginx -o jsonpath='{.spec.containers[0].image}'" "nginx@sha256:"
# Image lookup works for jobs
os::cmd::expect_success          "oc run --generator=job/v1 --restart=Never --image=nginx:latest nginx"
os::cmd::expect_success_and_text "oc get job/nginx -o jsonpath='{.spec.template.spec.containers[0].image}'" "nginx@sha256:"
# Image lookup works for replica sets
os::cmd::expect_success          "oc create deployment --image=nginx:latest nginx"
os::cmd::expect_success_and_text "oc get rs -o jsonpath='{..spec.template.spec.containers[0].image}'" "nginx@sha256:"
# Image lookup works for replication controllers
rc='{"kind":"ReplicationController","apiVersion":"v1","metadata":{"name":"nginx"},"spec":{"template":{"metadata":{"labels":{"app":"test"}},"spec":{"containers":[{"name":"main","image":"nginx:latest"}]}}}}'
os::cmd::expect_success          "echo '${rc}' | oc create -f -"
os::cmd::expect_success_and_text "oc get rc/nginx -o jsonpath='{.spec.template.spec.containers[0].image}'" "nginx@sha256:"

# Verify swapping settings on image stream
os::cmd::expect_success_and_text "oc set image-lookup is/nginx" "was not changed"
os::cmd::expect_success_and_text "oc set image-lookup nginx" "was not changed"
os::cmd::expect_success_and_text "oc set image-lookup is --list" "nginx.*true"
os::cmd::expect_success_and_text "oc set image-lookup nginx --enabled=false" "updated"
os::cmd::expect_success_and_text "oc set image-lookup is --list" "nginx.*false"
os::cmd::expect_failure_and_text "oc set image-lookup unknown --list" "the server doesn't have a resource type"
os::cmd::expect_success_and_text "oc set image-lookup secrets --list" "false"

# Clear resources
os::cmd::expect_success "oc delete deploy,dc,rs,rc,pods --all"

# Resource annotated with image lookup will create pods that resolve
os::cmd::expect_success          "oc tag nginx:latest alternate:latest"
rc='{"kind":"Deployment","apiVersion":"apps/v1beta1","metadata":{"name":"alternate"},"spec":{"template":{"metadata":{"labels":{"app":"test"}},"spec":{"containers":[{"name":"main","image":"alternate:latest"}]}}}}'
os::cmd::expect_success          "echo '${rc}' | oc set image-lookup -f - -o json | oc create -f -"
os::cmd::expect_success          "oc run --generator=run-pod/v1 --restart=Never --image=alternate:latest alternate"
os::cmd::expect_success_and_text "oc get pod/alternate -o jsonpath='{.spec.containers[0].image}'" "alternate:latest"
os::cmd::expect_success_and_text "oc get rs -o jsonpath='{..spec.template.spec.containers[0].image}'" "nginx@sha256:"

os::test::junit::declare_suite_end
