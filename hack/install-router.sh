#!/bin/bash
set -e

echo "Creating router file and starting pod..."

# ID to be used as the k8s id and also appended to the container name
ROUTER_ID="${1}"
# IP address to connect to the master, :8080 will be automatically appended
MASTER_IP="${2}"
# openshift executable - optional, will try to find it on the path if not specified
OPENSHIFT="${3}"

OS_ROOT=$(dirname "${BASH_SOURCE}")/..

if [[ "${OPENSHIFT}" == "" ]]; then
    if [[ "$(which osc)" != "" ]]; then
        OPENSHIFT=$(which osc)
    fi
fi

# update the template file
cp ${OS_ROOT}/images/router/haproxy/pod.json /tmp/router.json
sed -i s/MASTER_IP/${MASTER_IP}/ /tmp/router.json
sed -i s/ROUTER_ID/${ROUTER_ID}/g /tmp/router.json


# create the pod if we can find openshift
if [ "${OPENSHIFT}" == "" ]; then
    echo "unable to find openshift binary"
    echo "/tmp/router.json has been created.  In order to start the router please run:"
    echo "openshift kubectl create -f /tmp/router.json"
else
    "${OPENSHIFT}" create -f /tmp/router.json
fi
