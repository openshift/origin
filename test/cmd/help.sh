#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

# This test validates the help commands and output text

# verify some default commands
os::cmd::expect_success 'openshift'
os::cmd::expect_success 'openshift cli'
os::cmd::expect_success 'openshift ex'
os::cmd::expect_success 'openshift admin config'
os::cmd::expect_success 'openshift cli config'
os::cmd::expect_success 'openshift ex tokens'
os::cmd::expect_success 'openshift admin policy '
os::cmd::expect_success 'openshift kubectl'
os::cmd::expect_success 'openshift kube'
os::cmd::expect_success 'openshift admin'
os::cmd::expect_success 'openshift start kubernetes'
os::cmd::expect_success 'kubernetes'
os::cmd::expect_success 'kubectl'
os::cmd::expect_success 'oc'
os::cmd::expect_success 'osc'
os::cmd::expect_success 'oadm'
os::cmd::expect_success 'oadm'
os::cmd::expect_success 'origin'

# help for root commands must be consistent
os::cmd::expect_success_and_text 'openshift' 'Application Platform'
os::cmd::expect_success_and_text 'oc' 'Developer and Administrator Client'
os::cmd::expect_success_and_text 'oc' 'Build and Deploy Commands:'
os::cmd::expect_success_and_text 'oc' 'Other Commands:'
os::cmd::expect_success_and_text 'oc policy --help' 'add-role-to-user'
os::cmd::expect_success_and_not_text 'oc policy --help' 'Other Commands'
os::cmd::expect_success_and_not_text 'oc' 'Options'
os::cmd::expect_success_and_not_text 'oc' 'Global Options'
os::cmd::expect_success_and_text 'openshift cli' 'Developer and Administrator Client'
os::cmd::expect_success_and_text 'oc types' 'Deployment Config'
os::cmd::expect_success_and_text 'openshift kubectl' 'Kubernetes cluster'
os::cmd::expect_success_and_text 'oadm' 'Administrative Commands'
os::cmd::expect_success_and_text 'openshift admin' 'Administrative Commands'
os::cmd::expect_success_and_text 'oadm' 'Basic Commands:'
os::cmd::expect_success_and_text 'oadm' 'Install Commands:'
os::cmd::expect_success_and_text 'oadm ca' 'Manage certificates'
os::cmd::expect_success_and_text 'openshift start kubernetes' 'Kubernetes server components'
# check deprecated admin cmds for backward compatibility
os::cmd::expect_success_and_text 'oadm create-master-certs -h' 'Create keys and certificates'
os::cmd::expect_success_and_text 'oadm create-key-pair -h' 'Create an RSA key pair'
os::cmd::expect_success_and_text 'oadm create-server-cert -h' 'Create a key and server certificate'
os::cmd::expect_success_and_text 'oadm create-signer-cert -h' 'Create a self-signed CA'
# check whether product is recognized
os::cmd::expect_success_and_text 'origin' 'Origin Application Platform'
os::cmd::expect_success_and_text 'origin' 'Origin distribution of Kubernetes'
os::cmd::expect_success_and_not_text 'origin' '(Atomic|OpenShift)'
os::cmd::expect_success_and_text 'openshift' 'OpenShift Application Platform'
os::cmd::expect_success_and_text 'openshift' 'OpenShift distribution of Kubernetes'
os::cmd::expect_success_and_not_text 'openshift' 'Atomic'
os::cmd::expect_success_and_text 'atomic-enterprise' 'Atomic Enterprise Platform'
os::cmd::expect_success_and_text 'atomic-enterprise' 'Atomic distribution of Kubernetes'
os::cmd::expect_success_and_not_text 'atomic-enterprise' 'OpenShift'

# help for root commands with --help flag must be consistent
os::cmd::expect_success_and_text 'openshift --help' 'OpenShift Application Platform'
os::cmd::expect_success_and_text 'oc --help' 'Developer and Administrator Client'
os::cmd::expect_success_and_text 'oc login --help' 'Options'
os::cmd::expect_success_and_not_text 'oc login --help' 'Global Options'
os::cmd::expect_success_and_text 'oc login --help' 'insecure-skip-tls-verify'
os::cmd::expect_success_and_text 'openshift cli --help' 'Developer and Administrator Client'
os::cmd::expect_success_and_text 'openshift kubectl --help' 'Kubernetes cluster'
os::cmd::expect_success_and_text 'oadm --help' 'Administrative Commands'
os::cmd::expect_success_and_text 'openshift admin --help' 'Administrative Commands'

# help for root commands through help command must be consistent
os::cmd::expect_success_and_text 'openshift help cli' 'Developer and Administrator Client'
os::cmd::expect_success_and_text 'openshift help kubectl' 'Kubernetes cluster'
os::cmd::expect_success_and_text 'openshift help admin' 'Administrative Commands'

# help for given command with --help flag must be consistent
os::cmd::expect_success_and_text 'oc get --help' 'Display one or many resources'
os::cmd::expect_success_and_text 'openshift cli get --help' 'Display one or many resources'
os::cmd::expect_success_and_text 'openshift kubectl get --help' 'Display one or many resources'
os::cmd::expect_success_and_text 'openshift start --help' 'Start an all-in-one server'
os::cmd::expect_success_and_text 'openshift start master --help' 'Start a master'
os::cmd::expect_success_and_text 'openshift start node --help' 'Start a node'
os::cmd::expect_success_and_text 'oc project --help' 'Switch to another project'
os::cmd::expect_success_and_text 'oc projects --help' 'Switch to another project'
os::cmd::expect_success_and_text 'openshift cli project --help' 'Switch to another project'
os::cmd::expect_success_and_text 'openshift cli projects --help' 'Switch to another project'
os::cmd::expect_success_and_text 'oc get --help' 'oc'

# help for given command through help command must be consistent
os::cmd::expect_success_and_text 'oc help get' 'Display one or many resources'
os::cmd::expect_success_and_text 'openshift help cli get' 'Display one or many resources'
os::cmd::expect_success_and_text 'openshift help kubectl get' 'Display one or many resources'
os::cmd::expect_success_and_text 'openshift help start' 'Start an all-in-one server'
os::cmd::expect_success_and_text 'openshift help start master' 'Start a master'
os::cmd::expect_success_and_text 'openshift help start node' 'Start a node'
os::cmd::expect_success_and_text 'oc help project' 'Switch to another project'
os::cmd::expect_success_and_text 'oc help projects' 'Switch to another project'
# TODO: fix these tests
# os::cmd::expect_success_and_text 'openshift cli help update' 'Update a resource'
# os::cmd::expect_success_and_text 'openshift cli help replace' 'Replace a resource'
# os::cmd::expect_success_and_text 'openshift cli help patch' 'Update field\(s\) of a resource'
# os::cmd::expect_success_and_text 'openshift cli help project' 'Switch to another project'
# os::cmd::expect_success_and_text 'openshift cli help projects' 'Switch to another project'

# runnable commands with required flags must error consistently
os::cmd::expect_failure_and_text 'oc get' 'Required resource not specified'
os::cmd::expect_failure_and_text 'openshift cli get' 'Required resource not specified'
os::cmd::expect_failure_and_text 'openshift kubectl get' 'Required resource not specified'

# commands that expect file paths must validate and error out correctly
os::cmd::expect_failure_and_text 'oc login --certificate-authority=/path/to/invalid' 'no such file or directory'

# make sure that typoed commands come back with non-zero return codes
os::cmd::expect_failure 'openshift admin policy TYPO'
os::cmd::expect_failure 'openshift admin TYPO'
os::cmd::expect_failure 'openshift cli TYPO'
os::cmd::expect_failure 'oc policy TYPO'
os::cmd::expect_failure 'oc secrets TYPO'

# make sure that LDAP group sync and prune exist under both parents
os::cmd::expect_success_and_text 'openshift ex sync-groups --help' 'external provider'
os::cmd::expect_success_and_text 'openshift ex prune-groups --help' 'external provider'
os::cmd::expect_success_and_text 'openshift admin groups sync --help' 'external provider'
os::cmd::expect_success_and_text 'openshift admin groups prune --help' 'external provider'
os::cmd::expect_success_and_text 'openshift admin prune groups --help' 'external provider'