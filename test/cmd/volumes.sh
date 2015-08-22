#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# This test validates the 'volume' command

oc create -f test/integration/fixtures/test-deployment-config.json

[ "$(oc volume dc/test-deployment-config --list | grep vol1)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol0 -m /opt5)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol2 --type=emptydir -m /opt)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol1 --type=secret --secret-name='$ecret' -m /data 2>&1 | grep overwrite)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol1 --type=emptyDir -m /data --overwrite)" ]
[ "$(oc volume dc/test-deployment-config --add -m /opt 2>&1 | grep exists)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol2 -m /etc -c 'ruby' --overwrite 2>&1 | grep warning)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol2 -m /etc -c 'ruby*' --overwrite)" ]
[ "$(oc volume dc/test-deployment-config --list --name=vol2 | grep /etc)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol3 -o yaml | grep vol3)" ]
[ "$(oc volume dc/test-deployment-config --list --name=vol3 2>&1 | grep 'not found')" ]
[ "$(oc volume dc/test-deployment-config --remove 2>&1 | grep confirm)" ]
[ "$(oc volume dc/test-deployment-config --remove --name=vol2)" ]
[ ! "$(oc volume dc/test-deployment-config --list | grep vol2)" ]
[ "$(oc volume dc/test-deployment-config --remove --confirm)" ]
[ ! "$(oc volume dc/test-deployment-config --list | grep vol1)" ]

[ "$(oc get pvc --no-headers | wc -l)" -eq 0 ]
oc volume dc/test-deployment-config --add --mount-path=/other --claim-size=1G
oc volume dc/test-deployment-config --add --mount-path=/second --type=pvc --claim-size=1G --claim-mode=rwo
[ "$(oc get pvc --no-headers | wc -l)" -eq 2 ]

oc delete dc/test-deployment-config
echo "volumes: ok"