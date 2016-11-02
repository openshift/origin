#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

# Cleanup cluster resources created by this test
(
  set +e
  oc delete all,templates --all
  oc delete user someval
  exit 0
) &>/dev/null

os::test::junit::declare_suite_start "cmd/process"
# This test validates oc process

# fail to process two templates by name
os::cmd::expect_failure_and_text 'oc process name1 name2' 'template name must be specified only once'
# fail to pass a filename or template by name
os::cmd::expect_failure_and_text 'oc process' 'Must pass a filename or name of stored template'
# can't ask for parameters and try process the template
os::cmd::expect_failure_and_text 'oc process template-name --parameters --value=someval' '\-\-parameters flag does not process the template, can.t be used with \-\-value'
os::cmd::expect_failure_and_text 'oc process template-name --parameters -v someval' '\-\-parameters flag does not process the template, can.t be used with \-\-value'
os::cmd::expect_failure_and_text 'oc process template-name --parameters --labels=someval' '\-\-parameters flag does not process the template, can.t be used with \-\-labels'
os::cmd::expect_failure_and_text 'oc process template-name --parameters -l someval' '\-\-parameters flag does not process the template, can.t be used with \-\-labels'
os::cmd::expect_failure_and_text 'oc process template-name --parameters --output=someval' '\-\-parameters flag does not process the template, can.t be used with \-\-output'
os::cmd::expect_failure_and_text 'oc process template-name --parameters -o someval' '\-\-parameters flag does not process the template, can.t be used with \-\-output'
os::cmd::expect_failure_and_text 'oc process template-name --parameters --output-version=someval' '\-\-parameters flag does not process the template, can.t be used with \-\-output-version'
os::cmd::expect_failure_and_text 'oc process template-name --parameters --raw' '\-\-parameters flag does not process the template, can.t be used with \-\-raw'
os::cmd::expect_failure_and_text 'oc process template-name --parameters --template=someval' '\-\-parameters flag does not process the template, can.t be used with \-\-template'
os::cmd::expect_failure_and_text 'oc process template-name --parameters -t someval' '\-\-parameters flag does not process the template, can.t be used with \-\-template'

# providing a value more than once should fail
os::cmd::expect_failure_and_text 'oc process template-name key=value key=value' 'provided more than once: key'
os::cmd::expect_failure_and_text 'oc process template-name --value=key=value --value=key=value' 'provided more than once: key'
os::cmd::expect_failure_and_text 'oc process template-name key=value --value=key=value' 'provided more than once: key'
os::cmd::expect_failure_and_text 'oc process template-name key=value other=foo --value=key=value --value=other=baz' 'provided more than once: key, other'

required_params="${OS_ROOT}/test/testdata/template_required_params.yaml"

# providing something other than a template is not OK
os::cmd::expect_failure_and_text "oc process -f '${OS_ROOT}/test/testdata/basic-users-binding.json'" 'not a valid Template but'

# not providing required parameter should fail
os::cmd::expect_failure_and_text "oc process -f '${required_params}'" 'parameter required_param is required and must be specified'
# not providing an optional param is OK
os::cmd::expect_success "oc process -f '${required_params}' --value=required_param=someval | oc create -f -"
# parameters with multiple equal signs are OK
os::cmd::expect_success "oc process -f '${required_params}' required_param=someval=moreval | oc create -f -"
# we should have overwritten the template param
os::cmd::expect_success_and_text 'oc get user someval -o jsonpath={.Name}' 'someval'
# providing a value not in the template should fail
os::cmd::expect_failure_and_text "oc process -f '${required_params}' --value=required_param=someval --value=other_param=otherval" 'unknown parameter name "other_param"'
# failure on values fails the entire call
os::cmd::expect_failure_and_text "oc process -f '${required_params}' --value=required_param=someval --value=optional_param" 'invalid parameter assignment in'
# failure on labels fails the entire call
os::cmd::expect_failure_and_text "oc process -f '${required_params}' --value=required_param=someval --labels======" 'error parsing labels'

# values are not split on commas, required parameter is not recognized
os::cmd::expect_failure_and_text "oc process -f '${required_params}' --value=optional_param=a,required_param=b" 'parameter required_param is required and must be specified'
# warning is printed iff --value looks like two k-v pairs separated by comma
os::cmd::expect_success_and_text "oc process -f '${required_params}' --value=required_param=a,b=c,d" 'no longer accepts comma-separated list'
os::cmd::expect_success_and_not_text "oc process -f '${required_params}' --value=required_param=a_b_c_d" 'no longer accepts comma-separated list'
os::cmd::expect_success_and_not_text "oc process -f '${required_params}' --value=required_param=a,b,c,d" 'no longer accepts comma-separated list'
# warning is not printed for template values passed as positional arguments
os::cmd::expect_success_and_not_text "oc process -f '${required_params}' required_param=a,b=c,d" 'no longer accepts comma-separated list'

# set template parameter to contents of file
os::cmd::expect_success_and_text "oc process -f '${required_params}' --value=required_param='`cat ${OS_ROOT}/test/testdata/multiline.txt`'" 'also,with=commas'


echo "process: ok"
os::test::junit::declare_suite_end