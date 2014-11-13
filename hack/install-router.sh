#!/bin/bash
set -e

echo "Creating router file and starting pod..."

MASTER_IP="${1}"
OPENSHIFT="${2}"
OS_ROOT=$(dirname "${BASH_SOURCE}")/..

if [[ "${OPENSHIFT}" == "" ]]; then
    if [[ "$(which openshift)" != "" ]]; then
        OPENSHIFT=$(which openshift)
    fi
fi

# update the template file
cp ${OS_ROOT}/images/router/haproxy/pod.json /tmp/router.json
sed -i s/MASTER_IP/${MASTER_IP}/ /tmp/router.json


# create the pod if we can find openshift
if [ "${OPENSHIFT}" == "" ]; then
    echo "unable to find openshift binary"
    echo "/tmp/router.json has been created.  In order to start the router please run:"
    echo "openshift kube -c /tmp/router.json create pods"
else
    "${OPENSHIFT}" kube -c /tmp/router.json create pods
fi
