#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

# This test validates the 'volume' command

os::cmd::expect_success 'oc create -f test/integration/fixtures/test-deployment-config.json'

os::cmd::expect_success_and_text 'oc volume dc/test-deployment-config --list' 'vol1'
os::cmd::expect_success 'oc volume dc/test-deployment-config --add --name=vol0 -m /opt5'
os::cmd::expect_success 'oc volume dc/test-deployment-config --add --name=vol2 --type=emptydir -m /opt'
os::cmd::expect_failure_and_text "oc volume dc/test-deployment-config --add --name=vol1 --type=secret --secret-name='\$ecret' -m /data" 'overwrite to replace'
os::cmd::expect_success 'oc volume dc/test-deployment-config --add --name=vol1 --type=emptyDir -m /data --overwrite'
os::cmd::expect_failure_and_text 'oc volume dc/test-deployment-config --add -m /opt' "'/opt' already exists"
os::cmd::expect_success_and_text "oc volume dc/test-deployment-config --add --name=vol2 -m /etc -c 'ruby' --overwrite" 'does not have any containers'
os::cmd::expect_success "oc volume dc/test-deployment-config --add --name=vol2 -m /etc -c 'ruby*' --overwrite"
os::cmd::expect_success_and_text 'oc volume dc/test-deployment-config --list --name=vol2' 'mounted at /etc'
os::cmd::expect_success_and_text 'oc volume dc/test-deployment-config --add --name=vol3 -o yaml' 'name: vol3'
os::cmd::expect_failure_and_text 'oc volume dc/test-deployment-config --list --name=vol3' 'volume "vol3" not found'
os::cmd::expect_failure_and_text 'oc volume dc/test-deployment-config --remove' 'confirm for removing more than one volume'
os::cmd::expect_success 'oc volume dc/test-deployment-config --remove --name=vol2'
os::cmd::expect_success_and_not_text 'oc volume dc/test-deployment-config --list' 'vol2'
os::cmd::expect_success 'oc volume dc/test-deployment-config --remove --confirm'
os::cmd::expect_success_and_not_text 'oc volume dc/test-deployment-config --list' 'vol1'

os::cmd::expect_success_and_text 'oc get pvc --no-headers | wc -l' '0'
os::cmd::expect_success 'oc volume dc/test-deployment-config --add --mount-path=/other --claim-size=1G'
os::cmd::expect_success 'oc volume dc/test-deployment-config --add --mount-path=/second --type=pvc --claim-size=1G --claim-mode=rwo'
os::cmd::expect_success_and_text 'oc get pvc --no-headers | wc -l' '2'

# command alias
os::cmd::expect_success 'oc volumes --help'
os::cmd::expect_success 'oc volumes dc/test-deployment-config --list'

os::cmd::expect_success 'oc delete dc/test-deployment-config'
echo "volumes: ok"