#!/bin/bash

# This command checks that the built commands can function together for
# simple scenarios.  It does not require Docker so it can run in travis.

source $(dirname $0)/util.sh

function cleanup()
{
    set +e
    kill ${OS_PID} 1>&2 2>/dev/null
    echo
    echo "Complete"
}

trap cleanup EXIT SIGINT

set -e

ETCD_HOST=${ETCD_HOST:-127.0.0.1}
ETCD_PORT=${ETCD_PORT:-4001}
API_PORT=${API_PORT:-8080}
API_HOST=${API_HOST:-127.0.0.1}
KUBELET_PORT=${KUBELET_PORT:-10250}
GO_OUT=$(dirname $0)/../output/go/bin

# Check openshift version
out=$(${GO_OUT}/openshift version)
echo openshift: $out

# Start openshift
${GO_OUT}/openshift start 1>&2 &
OS_PID=$!

wait_for_url "http://127.0.0.1:${KUBELET_PORT}/healthz" "kubelet: "
wait_for_url "http://127.0.0.1:${API_PORT}/healthz" "apiserver: "

KUBE_CMD="${GO_OUT}/openshift kube -h http://127.0.0.1:${API_PORT} --expect_version_match"

${KUBE_CMD} list pods
echo "kubecfg(pods): ok"

${KUBE_CMD} list services
${KUBE_CMD} -c examples/test-service.json create services
${KUBE_CMD} delete services/frontend
echo "kubecfg(services): ok"

${KUBE_CMD} list minions
${KUBE_CMD} get minions/127.0.0.1
echo "kubecfg(minions): ok"
