#!/bin/sh

set -eu

if [ ! -f /etc/origin/master/master-config.yaml ]; then
    openshift start master --write-config=/etc/origin/master --master=${HOST_HOSTNAME}
fi

if [ ! -f /root/.kube/config ]; then
    mkdir -p /root/.kube
    cp /etc/origin/master/admin.kubeconfig /root/.kube/config
fi

exec openshift start master --config=/etc/origin/master/master-config.yaml --loglevel=4

