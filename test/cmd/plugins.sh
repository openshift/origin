#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/plugin"

# top-level plugin command
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc -h 2>&1" 'Runs a command-line plugin'

# no plugins
os::cmd::expect_failure_and_text "oc plugin 2>&1" 'no plugins installed'

# single plugins path
os::cmd::expect_failure_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin 2>&1" 'Echoes for test\-cmd'
os::cmd::expect_failure_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin 2>&1" 'The wonderful new plugin-based get!'
os::cmd::expect_failure_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin 2>&1" 'The tremendous plugin that always fails!'
os::cmd::expect_failure_and_not_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin 2>&1" 'The hello plugin'
os::cmd::expect_failure_and_not_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin 2>&1" 'Incomplete plugin'
os::cmd::expect_failure_and_not_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin 2>&1" 'no plugins installed'

# multiple plugins path
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins/:test/testdata/plugin/plugins2/ oc plugin -h 2>&1" 'Echoes for test-cmd'
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins/:test/testdata/plugin/plugins2/ oc plugin -h 2>&1" 'The wonderful new plugin-based get!'
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins/:test/testdata/plugin/plugins2/ oc plugin -h 2>&1" 'The tremendous plugin that always fails!'
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins/:test/testdata/plugin/plugins2/ oc plugin -h 2>&1" 'The hello plugin'
os::cmd::expect_success_and_not_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins/:test/testdata/plugin/plugins2/ oc plugin -h 2>&1" 'Incomplete plugin'

# don't override existing commands
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins/:test/testdata/plugin/plugins2/ oc get -h 2>&1" 'Display one or many resources'
os::cmd::expect_success_and_not_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins/:test/testdata/plugin/plugins2/ oc get -h 2>&1" 'The wonderful new plugin-based get'

# plugin help
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins/:test/testdata/plugin/plugins2/ oc plugin hello -h 2>&1" 'The hello plugin is a new plugin used by test-cmd to test multiple plugin locations.'
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins/:test/testdata/plugin/plugins2/ oc plugin hello -h 2>&1" 'Usage:'

# run plugin
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins/:test/testdata/plugin/plugins2/ oc plugin hello 2>&1" '#hello#'
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins/:test/testdata/plugin/plugins2/ oc plugin echo 2>&1" 'This plugin works!'
os::cmd::expect_failure_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin hello 2>&1" 'unknown command'
os::cmd::expect_failure_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin error 2>&1" 'error: exit status 1'

# plugin tree
os::cmd::expect_failure_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin tree 2>&1" 'Plugin with a tree of commands'
os::cmd::expect_failure_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin tree 2>&1" 'The first child of a tree'
os::cmd::expect_failure_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin tree 2>&1" 'The second child of a tree'
os::cmd::expect_failure_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin tree 2>&1" 'The third child of a tree'
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin tree child1 --help 2>&1" 'The first child of a tree'
os::cmd::expect_success_and_not_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin tree child1 --help 2>&1" 'The second child'
os::cmd::expect_success_and_not_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin tree child1 --help 2>&1" 'child2'
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin tree child1 2>&1" 'child one'
os::cmd::expect_success_and_not_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin tree child1 2>&1" 'child1'
os::cmd::expect_success_and_not_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin tree child1 2>&1" 'The first child'

# plugin env
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin env -h 2>&1" "This is a flag 1"
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin env -h 2>&1" "This is a flag 2"
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin env -h 2>&1" "This is a flag 3"
output_message=$()
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin env --test1=value1 -t value2 2>&1" 'KUBECTL_PLUGINS_CURRENT_NAMESPACE'
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin env --test1=value1 -t value2 2>&1" 'KUBECTL_PLUGINS_CALLER'
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin env --test1=value1 -t value2 2>&1" 'KUBECTL_PLUGINS_DESCRIPTOR_COMMAND=./env.sh'
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin env --test1=value1 -t value2 2>&1" 'KUBECTL_PLUGINS_DESCRIPTOR_SHORT_DESC=The plugin envs plugin'
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin env --test1=value1 -t value2 2>&1" 'KUBECTL_PLUGINS_GLOBAL_FLAG_CONFIG'
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin env --test1=value1 -t value2 2>&1" 'KUBECTL_PLUGINS_GLOBAL_FLAG_REQUEST_TIMEOUT=0'
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin env --test1=value1 -t value2 2>&1" 'KUBECTL_PLUGINS_LOCAL_FLAG_TEST1=value1'
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin env --test1=value1 -t value2 2>&1" 'KUBECTL_PLUGINS_LOCAL_FLAG_TEST2=value2'
os::cmd::expect_success_and_text "KUBECTL_PLUGINS_PATH=test/testdata/plugin/plugins oc plugin env --test1=value1 -t value2 2>&1" 'KUBECTL_PLUGINS_LOCAL_FLAG_TEST3=default'

echo "oc plugin: ok"
os::test::junit::declare_suite_end
