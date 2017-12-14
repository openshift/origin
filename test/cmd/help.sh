#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
trap os::test::junit::reconcile_output EXIT

os::test::junit::declare_suite_start "cmd/help"
# This test validates the help commands and output text

# verify some default commands
os::cmd::expect_failure 'openshift'
os::cmd::expect_success 'kubectl'
os::cmd::expect_success 'oc'
os::cmd::expect_success 'oc ex'
os::cmd::expect_failure 'origin'

# help for root commands must be consistent
os::cmd::expect_failure_and_text 'openshift' 'Application Platform'
os::cmd::expect_success_and_text 'oc' 'OpenShift Client'
os::cmd::expect_success_and_text 'oc -h' 'Build and Deploy Commands:'
os::cmd::expect_success_and_text 'oc -h' 'Other Commands:'
os::cmd::expect_success_and_text 'oc policy --help' 'add-role-to-user'
os::cmd::expect_success_and_not_text 'oc policy --help' 'Other Commands'
os::cmd::expect_success_and_not_text 'oc -h' 'Options'
os::cmd::expect_success_and_not_text 'oc -h' 'Global Options'
os::cmd::expect_failure_and_text 'oc types' 'Deployment Config'
os::cmd::expect_failure_and_text 'oc adm ca' 'Manage certificates'
os::cmd::expect_success_and_text 'oc exec --help' '\[options\] POD \[\-c CONTAINER\] \-\- COMMAND \[args\.\.\.\]$'
os::cmd::expect_success_and_text 'oc rsh --help' '\[options\] POD \[COMMAND\]$'

# check deprecated admin cmds for backward compatibility
os::cmd::expect_success_and_text 'oc adm create-master-certs -h' 'Create keys and certificates'
os::cmd::expect_success_and_text 'oc adm create-key-pair -h' 'Create an RSA key pair'
os::cmd::expect_success_and_text 'oc adm create-server-cert -h' 'Create a key and server certificate'
os::cmd::expect_success_and_text 'oc adm create-signer-cert -h' 'Create a self-signed CA'

# help for root commands with --help flag must be consistent
os::cmd::expect_success_and_text 'openshift --help' 'OpenShift Application Platform'
os::cmd::expect_success_and_text 'oc --help' 'OpenShift Client'
os::cmd::expect_success_and_text 'oc login --help' 'Options'
os::cmd::expect_success_and_not_text 'oc login --help' 'Global Options'
os::cmd::expect_success_and_text 'oc login --help' 'insecure-skip-tls-verify'

# help for given command with --help flag must be consistent
os::cmd::expect_success_and_text 'oc get --help' 'Display one or many resources'
os::cmd::expect_success_and_text 'openshift start --help' 'Start an all-in-one server'
os::cmd::expect_success_and_text 'openshift start master --help' 'Start a master'
os::cmd::expect_success_and_text 'openshift start node --help' 'Start a node'
os::cmd::expect_success_and_text 'oc project --help' 'Switch to another project'
os::cmd::expect_success_and_text 'oc projects --help' 'existing projects'
os::cmd::expect_success_and_text 'oc get --help' 'oc'

# help for given command through help command must be consistent
os::cmd::expect_success_and_text 'oc help get' 'Display one or many resources'
os::cmd::expect_success_and_text 'openshift help start' 'Start an all-in-one server'
os::cmd::expect_success_and_text 'openshift help start master' 'Start a master'
os::cmd::expect_success_and_text 'openshift help start node' 'Start a node'
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
os::cmd::expect_success_and_text 'oc ex sync-groups --help' 'external provider'
os::cmd::expect_success_and_text 'oc ex prune-groups --help' 'external provider'
os::cmd::expect_success_and_text 'oc adm groups sync --help' 'external provider'
os::cmd::expect_success_and_text 'oc adm groups prune --help' 'external provider'
os::cmd::expect_success_and_text 'oc adm prune groups --help' 'external provider'

os::test::junit::declare_suite_end