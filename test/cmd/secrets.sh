#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# This test validates secret interaction

oc secrets new-dockercfg dockercfg --docker-username=sample-user --docker-password=sample-password --docker-email=fake@example.org
# can't use a go template here because the output needs to be base64 decoded.  base64 isn't installed by default in all distros
oc describe secrets/dockercfg | grep "dockercfg:" | awk '{print $2}' > ${HOME}/dockerconfig
oc secrets new from-file .dockercfg=${HOME}/dockerconfig
# check to make sure the type was correctly auto-detected
[ "$(oc get secret/from-file -t "{{ .type }}" | grep 'kubernetes.io/dockercfg')" ]
# make sure the -o works correctly
[ "$(oc secrets new-dockercfg dockercfg --docker-username=sample-user --docker-password=sample-password --docker-email=fake@example.org -o yaml | grep "kubernetes.io/dockercfg")" ]
[ "$(oc secrets new from-file .dockercfg=${HOME}/dockerconfig -o yaml | grep "kubernetes.io/dockercfg")" ]
# check to make sure malformed names fail as expected
[ "$(oc secrets new bad-name .docker=cfg=${HOME}/dockerconfig 2>&1 | grep "error: Key names or file paths cannot contain '='.")" ] 


# attach secrets to service account
# single secret with prefix
oc secrets add serviceaccounts/deployer secrets/dockercfg
# don't add the same secret twice
oc secrets add serviceaccounts/deployer secrets/dockercfg secrets/from-file
# make sure we can add as as pull secret
oc secrets add serviceaccounts/deployer secrets/dockercfg secrets/from-file --for=pull
# make sure we can add as as pull secret and mount secret at once
oc secrets add serviceaccounts/deployer secrets/dockercfg secrets/from-file --for=pull,mount
echo "secrets: ok"

