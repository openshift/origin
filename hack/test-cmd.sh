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

clients=( "$CLI_CMD" )

for cli in "${clients[@]}"
do
  ${cli} get pods
  ${cli} create -f examples/hello-openshift/hello-pod.json
  ${cli} delete pods hello-openshift
  echo "kube(pods): ok"

  ${cli} get services
  ${cli} create -f test/integration/fixtures/test-service.json
  ${cli} delete services frontend
  echo "kube(services): ok"

  ${cli} get minions
  echo "kube(minions): ok"

  ${cli} get images
  ${cli} create -f test/integration/fixtures/test-image.json
  ${cli} delete images test
  echo "kube(images): ok"

  ${cli} get imageRepositories
  ${cli} create -f test/integration/fixtures/test-image-repository.json
  ${cli} delete imageRepositories test
  echo "kube(imageRepositories): ok"

  ${cli} create -f test/integration/fixtures/test-image-repository.json
  ${cli} create -f test/integration/fixtures/test-mapping.json
  ${cli} get images
  ${cli} get imageRepositories
  echo "kube(imageRepositoryMappings): ok"

  ${cli} get routes
  ${cli} create -f test/integration/fixtures/test-route.json create routes
  ${cli} delete routes testroute
  echo "kube(routes): ok"

  ${cli} get deploymentConfigs
  ${cli} create -f test/integration/fixtures/test-deployment-config.json
  ${cli} delete deploymentConfigs test-deployment-config
  echo "kube(deploymentConfigs): ok"

  ${cli} process -f examples/guestbook/template.json | ${cli} apply -f -
  echo "kube(template+config): ok"

  ${cli} process -f examples/sample-app/application-template-dockerbuild.json | ${cli} apply -f -
  echo "kube(buildConfig): ok"

  ${cli} start-build ruby-sample-build
  echo "kube(start-build): ok"
done
