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

CLI_CMD="${GO_OUT}/openshift cli --server=http://${API_HOST}:${API_PORT} --match-server-version"

${CLI_CMD} get pods
${CLI_CMD} create -f examples/hello-openshift/hello-pod.json
${CLI_CMD} delete pods hello-openshift
echo "pods: ok"

${CLI_CMD} get services
${CLI_CMD} create -f test/integration/fixtures/test-service.json
${CLI_CMD} delete services frontend
echo "services: ok"

${CLI_CMD} get minions
echo "minions: ok"

${CLI_CMD} get images
${CLI_CMD} create -f test/integration/fixtures/test-image.json
${CLI_CMD} delete images test
echo "images: ok"

${CLI_CMD} get imageRepositories
${CLI_CMD} create -f test/integration/fixtures/test-image-repository.json
${CLI_CMD} delete imageRepositories test
echo "imageRepositories: ok"

${CLI_CMD} create -f test/integration/fixtures/test-image-repository.json
${CLI_CMD} create -f test/integration/fixtures/test-mapping.json
${CLI_CMD} get images
${CLI_CMD} get imageRepositories
${CLI_CMD} delete imageRepositories test
echo "imageRepositoryMappings: ok"

${CLI_CMD} get routes
${CLI_CMD} create -f test/integration/fixtures/test-route.json create routes
${CLI_CMD} delete routes testroute
echo "routes: ok"

${CLI_CMD} get deploymentConfigs
${CLI_CMD} create -f test/integration/fixtures/test-deployment-config.json
${CLI_CMD} delete deploymentConfigs test-deployment-config
echo "deploymentConfigs: ok"

${CLI_CMD} process -f examples/guestbook/template.json | ${CLI_CMD} apply -f -
echo "template+config: ok"

${CLI_CMD} process -f examples/sample-app/application-template-dockerbuild.json | ${CLI_CMD} apply -f -
echo "buildConfig: ok"

${CLI_CMD} start-build ruby-sample-build
echo "start-build: ok"

BUILD_NAME=`${CLI_CMD} get builds | awk 'NR==2'  | awk '{print $1}'`
${CLI_CMD} cancel-build ${BUILD_NAME} --dump-logs --restart
echo "cancel-build: ok"
