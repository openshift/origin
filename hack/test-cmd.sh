#!/bin/bash

# This command checks that the built commands can function together for
# simple scenarios.  It does not require Docker so it can run in travis.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

function cleanup()
{
    if [ $? -ne 0 ]; then
      echo "[FAIL] !!!!! Test Failed !!!!"
    fi
    set +e
    kill ${OS_PID-} 1>&2 2>/dev/null
    echo
    echo "Complete"
}

trap cleanup EXIT SIGINT

set -e

USE_LOCAL_IMAGES=${USE_LOCAL_IMAGES:-true}

ETCD_HOST=${ETCD_HOST:-127.0.0.1}
ETCD_PORT=${ETCD_PORT:-4001}
API_PORT=${API_PORT:-8080}
API_HOST=${API_HOST:-127.0.0.1}
KUBELET_PORT=${KUBELET_PORT:-10250}
GO_OUT="${OS_ROOT}/_output/local/go/bin"

ETCD_DATA_DIR=$(mktemp -d /tmp/openshift.local.etcd.XXXX)
VOLUME_DIR=$(mktemp -d /tmp/openshift.local.volumes.XXXX)

# Check openshift version
out=$(${GO_OUT}/openshift version)
echo openshift: $out

# Start openshift
${GO_OUT}/openshift start --master="${API_HOST}:${API_PORT}" --volume-dir="${VOLUME_DIR}" --etcd-dir="${ETCD_DATA_DIR}" 1>&2 &
OS_PID=$!

wait_for_url "http://localhost:${KUBELET_PORT}/healthz" "kubelet: " 1 30
wait_for_url "http://${API_HOST}:${API_PORT}/healthz" "apiserver: "

# TODO: Replace KUBE_CMD with KUBECTL_CMD
KUBE_CMD="${GO_OUT}/openshift kubectl --server=http://${API_HOST}:${API_PORT} --match-server-version"

${KUBE_CMD} get pods
${KUBE_CMD} create -f examples/hello-openshift/hello-pod.json
${KUBE_CMD} delete pods hello-openshift
echo "kube(pods): ok"

${KUBE_CMD} get services
${KUBE_CMD} create -f test/integration/fixtures/test-service.json
${KUBE_CMD} delete services frontend
echo "kube(services): ok"

${KUBE_CMD} get minions
echo "kube(minions): ok"

${KUBE_CMD} get images
${KUBE_CMD} create -f test/integration/fixtures/test-image.json
${KUBE_CMD} delete images test
echo "kube(images): ok"

${KUBE_CMD} get imageRepositories
${KUBE_CMD} create -f test/integration/fixtures/test-image-repository.json
${KUBE_CMD} delete imageRepositories test
echo "kube(imageRepositories): ok"

${KUBE_CMD} create -f test/integration/fixtures/test-image-repository.json
${KUBE_CMD} create -f test/integration/fixtures/test-mapping.json
${KUBE_CMD} get images
${KUBE_CMD} get imageRepositories
echo "kube(imageRepositoryMappings): ok"

${KUBE_CMD} get routes
${KUBE_CMD} create -f test/integration/fixtures/test-route.json create routes
${KUBE_CMD} delete routesa testroute
echo "kube(routes): ok"

${KUBE_CMD} get deploymentConfigs
${KUBE_CMD} create -f test/integration/fixtures/test-deployment-config.json
${KUBE_CMD} delete deploymentConfigs test-deployment-config
echo "kube(deploymentConfigs): ok"

${KUBE_CMD} process -f examples/guestbook/template.json | ${KUBE_CMD} apply -f -
echo "kube(template+config): ok"
