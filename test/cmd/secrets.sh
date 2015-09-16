#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

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

GIT_CONFIG_PATH=$(create_gitconfig)
CA_CERT_PATH=$(create_valid_file ca.pem)
PRIVATE_KEY_PATH=$(create_valid_file id_rsa)

oc secrets new-basicauth basicauth --username=sample-user --password=sample-password --gitconfig=$GIT_CONFIG_PATH --ca-cert=$PRIVATE_KEY_PATH
# check to make sure two mutual exclusive flags return error as expected
[ "$(oc secrets new-basicauth bad-file --password=sample-password --prompt 2>&1 | grep "error: must provide either --prompt or --password flag")" ]
# check to make sure incorrect .gitconfig path fail as expected
[ "$(oc secrets new-basicauth bad-file --username=user --gitconfig=/bad/path 2>&1 | grep "error: open /bad/path: no such file or directory")" ]

oc secrets new-sshauth sshauth --ssh-privatekey=$PRIVATE_KEY_PATH --ca-cert=$PRIVATE_KEY_PATH
# check to make sure incorrect SSH private-key path fail as expected
[ "$(oc secrets new-sshauth bad-file --ssh-privatekey=/bad/path 2>&1 | grep "error: open /bad/path: no such file or directory")" ]

# attach secrets to service account
# single secret with prefix
oc secrets add serviceaccounts/deployer secrets/basicauth
# don't add the same secret twice
oc secrets add serviceaccounts/deployer secrets/basicauth secrets/sshauth
# make sure we can add as as pull secret
oc secrets add serviceaccounts/deployer secrets/basicauth secrets/sshauth --for=pull
# make sure we can add as as pull secret and mount secret at once
oc secrets add serviceaccounts/deployer secrets/basicauth secrets/sshauth --for=pull,mount

echo "secrets: ok"
