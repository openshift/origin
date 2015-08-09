#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# This test validates the 'volume' command

oc create -f test/integration/fixtures/test-deployment-config.json

[ "$(oc volume dc/test-deployment-config --list | grep vol1)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol2 -m /opt)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol1 --type=secret --secret-name='$ecret' -m /data | grep overwrite)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol1 --type=emptyDir -m /data --overwrite)" ]
[ "$(oc volume dc/test-deployment-config --add -m /opt | grep exists)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol2 -m /etc -c 'ruby' --overwrite | grep warning)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol2 -m /etc -c 'ruby*' --overwrite)" ]
[ "$(oc volume dc/test-deployment-config --list --name=vol2 | grep /etc)" ]
[ "$(oc volume dc/test-deployment-config --add --name=vol3 -o yaml | grep vol3)" ]
[ "$(oc volume dc/test-deployment-config --list --name=vol3 | grep 'not found')" ]
[ "$(oc volume dc/test-deployment-config --remove 2>&1 | grep confirm)" ]
[ "$(oc volume dc/test-deployment-config --remove --name=vol2)" ]
[ ! "$(oc volume dc/test-deployment-config --list | grep vol2)" ]
[ "$(oc volume dc/test-deployment-config --remove --confirm)" ]
[ ! "$(oc volume dc/test-deployment-config --list | grep vol1)" ]

oc delete dc/test-deployment-config
echo "volumes: ok"