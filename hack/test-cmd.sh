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

ETCD_DATA_DIR=$(mktemp -d /tmp/openshift.local.etcd.XXXX)
VOLUME_DIR=$(mktemp -d /tmp/openshift.local.volumes.XXXX)

# set path so OpenShift is available
GO_OUT="${OS_ROOT}/_output/local/go/bin"
export PATH="${GO_OUT}:${PATH}"
pushd "${GO_OUT}" > /dev/null
ln -fs "openshift" "osc"
popd > /dev/null

# Check openshift version
out=$(openshift version)
echo openshift: $out

# Start openshift
openshift start --master="${API_HOST}:${API_PORT}" --volume-dir="${VOLUME_DIR}" --etcd-dir="${ETCD_DATA_DIR}" 1>&2 &
OS_PID=$!

wait_for_url "http://localhost:${KUBELET_PORT}/healthz" "kubelet: " 1 30
wait_for_url "http://${API_HOST}:${API_PORT}/healthz" "apiserver: "

export KUBERNETES_MASTER="${API_HOST}:${API_PORT}"

#
# Begin tests
#

osc get pods --match-server-version
osc create -f examples/hello-openshift/hello-pod.json
osc delete pods hello-openshift
echo "pods: ok"

osc get services
osc create -f test/integration/fixtures/test-service.json
osc delete services frontend
echo "services: ok"

osc get minions
echo "minions: ok"

osc get images
osc create -f test/integration/fixtures/test-image.json
osc delete images test
echo "images: ok"

osc get imageRepositories
osc create -f test/integration/fixtures/test-image-repository.json
osc delete imageRepositories test
echo "imageRepositories: ok"

osc create -f test/integration/fixtures/test-image-repository.json
osc create -f test/integration/fixtures/test-mapping.json
osc get images
osc get imageRepositories
osc delete imageRepositories test
echo "imageRepositoryMappings: ok"

osc get routes
osc create -f test/integration/fixtures/test-route.json create routes
osc delete routes testroute
echo "routes: ok"

osc get deploymentConfigs
osc create -f test/integration/fixtures/test-deployment-config.json
osc delete deploymentConfigs test-deployment-config
echo "deploymentConfigs: ok"

osc process -f examples/guestbook/template.json | osc apply -f -
echo "template+config: ok"

osc process -f examples/sample-app/application-template-dockerbuild.json | osc apply -f -
echo "buildConfig: ok"

started=$(osc start-build ruby-sample-build)
echo "start-build: ok"

osc cancel-build "${started}" --dump-logs --restart
echo "cancel-build: ok"
