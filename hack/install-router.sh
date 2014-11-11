#!/bin/bash
set -e

echo "Creating router file and starting pod..."

MASTER_IP="${1}"
OS_ROOT=$(dirname "${BASH_SOURCE}")/..


# update the template file
cp ${OS_ROOT}/images/router/haproxy/pod.json /tmp/router.json
sed -i s/MASTER_IP/${MASTER_IP}/ /tmp/router.json


# create the pod if we can find openshift
if [ "$(which openshift)" == "" ]; then
    echo "unable to find openshift in your PATH"
    echo "/tmp/router.json has been created.  In order to start the router please run:"
    echo "openshift kube -c /tmp/router.json create pods"
else
    openshift kube -c /tmp/router.json create pods
fi
