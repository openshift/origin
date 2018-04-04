#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  oc delete customresourcedefinition idlers.idling.openshift.io
  exit 0
) &>/dev/null

project="$(oc project -q)"
dc_name=""

setup_idling_resources() {
    os::cmd::expect_success 'oc delete all --all'

    # set up resources for the idle command
    local idler_name
    idler_name=$(basename $(oc create -f test/testdata/idler.yaml -o name))  # `basename type/name` --> name
    os::cmd::expect_success "oc describe idler '${idler_name}'"
}

# run the actual tests
os::test::junit::declare_suite_start "cmd/idle/idle-and-unidle"

# set up the service idler and idling resources
os::cmd::expect_success 'oc create -f install/service-idler/service-idler.yaml'
setup_idling_resources  # set up per-suite resources

os::cmd::expect_failure "oc idle dc/foo" # make sure manually passing non-idler resources fails
os::cmd::expect_success_and_text 'oc idle idling-echo' 'idler.idling.openshift.io "idling-echo" idled'
os::cmd::expect_success_and_text "oc get idler idling-echo -o go-template='{{.spec.wantIdle}}'" "true"
os::cmd::expect_success_and_text 'oc idle --unidle idling-echo' 'idler.idling.openshift.io "idling-echo" unidled'
os::cmd::expect_success_and_text "oc get idler idling-echo -o go-template='{{.spec.wantIdle}}'" "false"
