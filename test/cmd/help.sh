#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# This test validates the help commands and output text

# verify some default commands
[ "$(openshift 2>&1)" ]
[ "$(openshift cli)" ]
[ "$(openshift ex)" ]
[ "$(openshift admin config 2>&1)" ]
[ "$(openshift cli config 2>&1)" ]
[ "$(openshift ex tokens)" ]
[ "$(openshift admin policy  2>&1)" ]
[ "$(openshift kubectl 2>&1)" ]
[ "$(openshift kube 2>&1)" ]
[ "$(openshift admin 2>&1)" ]
[ "$(openshift start kubernetes 2>&1)" ]
[ "$(kubernetes 2>&1)" ]
[ "$(kubectl 2>&1)" ]
[ "$(oc 2>&1)" ]
[ "$(osc 2>&1)" ]
[ "$(oadm 2>&1)" ]
[ "$(oadm 2>&1)" ]
[ "$(origin 2>&1)" ]

# help for root commands must be consistent
[ "$(openshift | grep 'Application Platform')" ]
[ "$(oc | grep 'Developer and Administrator Client')" ]
[ "$(oc | grep 'Build and Deploy Commands:')" ]
[ "$(oc | grep 'Other Commands:')" ]
[ "$(oc policy --help 2>&1 | grep 'add-role-to-user')" ]
[ ! "$(oc policy --help 2>&1 | grep 'Other Commands')" ]
[ ! "$(oc 2>&1 | grep 'Options')" ]
[ ! "$(oc 2>&1 | grep 'Global Options')" ]
[ "$(openshift cli 2>&1 | grep 'Developer and Administrator Client')" ]
[ "$(oc types | grep 'Deployment Config')" ]
[ "$(openshift kubectl 2>&1 | grep 'Kubernetes cluster')" ]
[ "$(oadm 2>&1 | grep 'Administrative Commands')" ]
[ "$(openshift admin 2>&1 | grep 'Administrative Commands')" ]
[ "$(oadm | grep 'Basic Commands:')" ]
[ "$(oadm | grep 'Install Commands:')" ]
[ "$(oadm ca | grep 'Manage certificates')" ]
[ "$(openshift start kubernetes 2>&1 | grep 'Kubernetes server components')" ]
# check deprecated admin cmds for backward compatibility
[ "$(oadm create-master-certs -h 2>&1 | grep 'Create keys and certificates')" ]
[ "$(oadm create-key-pair -h 2>&1 | grep 'Create an RSA key pair')" ]
[ "$(oadm create-server-cert -h 2>&1 | grep 'Create a key and server certificate')" ]
[ "$(oadm create-signer-cert -h 2>&1 | grep 'Create a self-signed CA')" ]
# check whether product is recognized
[ "$(origin | grep -i 'Origin Application Platform')" ]
[ "$(origin | grep -i 'Origin distribution of Kubernetes')" ]
[ ! "$(origin | grep -i '\(Atomic\|OpenShift\)')" ]
[ "$(openshift | grep -i 'OpenShift Application Platform')" ]
[ "$(openshift | grep -i 'OpenShift distribution of Kubernetes')" ]
[ ! "$(openshift | grep -i 'Atomic')" ]
[ "$(atomic-enterprise | grep -i 'Atomic Enterprise Platform')" ]
[ "$(atomic-enterprise | grep -i 'Atomic distribution of Kubernetes')" ]
[ ! "$(atomic-enterprise | grep -i 'OpenShift')" ]

# help for root commands with --help flag must be consistent
[ "$(openshift --help 2>&1 | grep 'OpenShift Application Platform')" ]
[ "$(oc --help 2>&1 | grep 'Developer and Administrator Client')" ]
[ "$(oc login --help 2>&1 | grep 'Options')" ]
[ ! "$(oc login --help 2>&1 | grep 'Global Options')" ]
[ "$(oc login --help 2>&1 | grep 'insecure-skip-tls-verify')" ]
[ "$(openshift cli --help 2>&1 | grep 'Developer and Administrator Client')" ]
[ "$(openshift kubectl --help 2>&1 | grep 'Kubernetes cluster')" ]
[ "$(oadm --help 2>&1 | grep 'Administrative Commands')" ]
[ "$(openshift admin --help 2>&1 | grep 'Administrative Commands')" ]

# help for root commands through help command must be consistent
[ "$(openshift help cli 2>&1 | grep 'Developer and Administrator Client')" ]
[ "$(openshift help kubectl 2>&1 | grep 'Kubernetes cluster')" ]
[ "$(openshift help admin 2>&1 | grep 'Administrative Commands')" ]

# help for given command with --help flag must be consistent
[ "$(oc get --help 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift cli get --help 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift kubectl get --help 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift start --help 2>&1 | grep 'Start an all-in-one server')" ]
[ "$(openshift start master --help 2>&1 | grep 'Start a master')" ]
[ "$(openshift start node --help 2>&1 | grep 'Start a node')" ]
[ "$(oc get --help 2>&1 | grep 'oc')" ]

# help for given command through help command must be consistent
[ "$(oc help get 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift help cli get 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift help kubectl get 2>&1 | grep 'Display one or many resources')" ]
[ "$(openshift help start 2>&1 | grep 'Start an all-in-one server')" ]
[ "$(openshift help start master 2>&1 | grep 'Start a master')" ]
[ "$(openshift help start node 2>&1 | grep 'Start a node')" ]
[ "$(openshift cli help update 2>&1 | grep 'openshift')" ]
[ "$(openshift cli help replace 2>&1 | grep 'openshift')" ]
[ "$(openshift cli help patch 2>&1 | grep 'openshift')" ]

# runnable commands with required flags must error consistently
[ "$(oc get 2>&1 | grep 'Required resource not specified')" ]
[ "$(openshift cli get 2>&1 | grep 'Required resource not specified')" ]
[ "$(openshift kubectl get 2>&1 | grep 'Required resource not specified')" ]

# commands that expect file paths must validate and error out correctly
[ "$(oc login --certificate-authority=/path/to/invalid 2>&1 | grep 'no such file or directory')" ]

# make sure that typoed commands come back with non-zero return codes
[ "$(openshift admin policy TYPO; echo $? | grep '1')" ]
[ "$(openshift admin TYPO; echo $? | grep '1')" ]
[ "$(openshift cli TYPO; echo $? | grep '1')" ]
[ "$(oc policy TYPO; echo $? | grep '1')" ]
[ "$(oc secrets TYPO; echo $? | grep '1')" ]
