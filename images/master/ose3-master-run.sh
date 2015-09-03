#!/bin/sh

set -eu

if [ ! -f /etc/openshift/master/master-config.yaml ]; then
    /usr/bin/openshift start master --write-config=/etc/openshift/master --master=${HOST_HOSTNAME}
fi

if [ ! -f /root/.kube/config ]; then
    mkdir -p /root/.kube
    cp /etc/openshift/master/admin.kubeconfig /root/.kube/config
fi

exec /usr/bin/openshift start master --config=/etc/openshift/master/master-config.yaml --loglevel=4

