#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/help"
# This test validates the help commands and output text

# verify some default commands
os::cmd::expect_success 'kubectl'
os::cmd::expect_success 'oc'
os::cmd::expect_success 'oc ex'
os::cmd::expect_failure 'origin'

# help for root commands must be consistent
os::cmd::expect_success_and_text 'oc' 'OpenShift Client'
os::cmd::expect_success_and_text 'oc -h' 'Build and Deploy Commands:'
os::cmd::expect_success_and_text 'oc -h' 'Other Commands:'
os::cmd::expect_success_and_text 'oc policy --help' 'add-role-to-user'
os::cmd::expect_success_and_not_text 'oc policy --help' 'Other Commands'
os::cmd::expect_success_and_not_text 'oc -h' 'Options'
os::cmd::expect_success_and_not_text 'oc -h' 'Global Options'
os::cmd::expect_failure_and_text 'oc adm ca' 'Manage certificates'
os::cmd::expect_success_and_text 'oc exec --help' '\-\- COMMAND \[args\.\.\.\]$'
os::cmd::expect_success_and_text 'oc rsh --help' 'COMMAND'

# help for root commands with --help flag must be consistent
os::cmd::expect_success_and_text 'oc --help' 'OpenShift Client'
os::cmd::expect_success_and_text 'oc login --help' 'Options'
os::cmd::expect_success_and_not_text 'oc login --help' 'Global Options'
os::cmd::expect_success_and_text 'oc login --help' 'insecure-skip-tls-verify'

# help for given command with --help flag must be consistent
os::cmd::expect_success_and_text 'oc get --help' 'Display one or many resources'
os::cmd::expect_success_and_text 'oc project --help' 'Switch to another project'
os::cmd::expect_success_and_text 'oc projects --help' 'existing projects'
os::cmd::expect_success_and_text 'oc get --help' 'oc'

# help for given command through help command must be consistent
os::cmd::expect_success_and_text 'oc help get' 'Display one or many resources'
os::cmd::expect_success_and_text 'oc help project' 'Switch to another project'
os::cmd::expect_success_and_text 'oc help projects' 'current active project and existing projects on the server'

# help tips must be consistent
os::cmd::expect_success_and_text 'oc --help' 'Use "oc <command> --help" for more information'
os::cmd::expect_success_and_text 'oc --help' 'Use "oc options" for a list of global'
os::cmd::expect_success_and_text 'oc help' 'Use "oc <command> --help" for more information'
os::cmd::expect_success_and_text 'oc help' 'Use "oc options" for a list of global'
os::cmd::expect_success_and_text 'oc set --help' 'Use "oc set <command> --help" for more information'
os::cmd::expect_success_and_text 'oc set --help' 'Use "oc options" for a list of global'
os::cmd::expect_success_and_text 'oc set env --help' 'Use "oc options" for a list of global'

# runnable commands with required flags must error consistently
os::cmd::expect_failure_and_text 'oc get' 'Required resource not specified'

# commands that expect file paths must validate and error out correctly
os::cmd::expect_failure_and_text 'oc login --certificate-authority=/path/to/invalid' 'no such file or directory'

# make sure that typoed commands come back with non-zero return codes
os::cmd::expect_failure 'oc policy TYPO'
os::cmd::expect_failure 'oc secrets TYPO'

# make sure that LDAP group sync and prune exist under both experimental and `oc adm`
os::cmd::expect_success_and_text 'oc adm groups sync --help' 'external provider'
os::cmd::expect_success_and_text 'oc adm groups prune --help' 'external provider'
os::cmd::expect_success_and_text 'oc adm prune groups --help' 'external provider'

os::test::junit::declare_suite_end
