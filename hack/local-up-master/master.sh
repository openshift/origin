#!/usr/bin/env bash

set -e

source "$(dirname "${BASH_SOURCE}")/../lib/init.sh"
source "$(dirname "${BASH_SOURCE}")/../local-up-master/lib.sh"

trap "clusterup::cleanup" EXIT

localup::init_master

echo
echo "Cluster is available, the following kubeconfig to interact with it"
echo "export KUBECONFIG=${CONFIG}/admin.kubeconfig"
echo "Press ctrl+C to finish"

while true; do sleep 1; localup::healthcheck; done