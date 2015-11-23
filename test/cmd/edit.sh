#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

# This test validates the edit command

os::cmd::expect_success 'oc create -f examples/hello-openshift/hello-pod.json'

os::cmd::expect_success_and_text 'OC_EDITOR=cat oc edit pod/hello-openshift' 'Edit cancelled'
os::cmd::expect_success_and_text 'OC_EDITOR=cat oc edit pod/hello-openshift' 'name: hello-openshift'
os::cmd::expect_success_and_text 'OC_EDITOR=cat oc edit --windows-line-endings pod/hello-openshift | file -' 'CRLF'
os::cmd::expect_success_and_not_text 'OC_EDITOR=cat oc edit --windows-line-endings=false pod/hello-openshift | file -' 'CRFL'
echo "edit: ok"

