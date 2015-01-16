#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app

# Default to use STI builder if no argument specified
BUILD_TYPE=${1:-sti}

if [[ -z "$(which iptables)" ]]; then
  echo "IPTables not found - the end-to-end test requires a system with iptables for Kubernetes services."
  exit 1
fi
iptables --list > /dev/null 2>&1
if [ $? -ne 0 ]; then
  sudo iptables --list > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    echo "You do not have iptables or sudo privileges.  Kubernetes services will not work without iptables access.  See https://github.com/GoogleCloudPlatform/kubernetes/issues/1859.  Try 'sudo hack/test-end-to-end.sh'."
    exit 1
  fi
fi

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

echo "[INFO] Starting end-to-end test using ${BUILD_TYPE} builder"

USE_LOCAL_IMAGES="${USE_LOCAL_IMAGES:-true}"
ROUTER_TESTS_ENABLED="${ROUTER_TESTS_ENABLED:-true}"

TMPDIR="${TMPDIR:-"/tmp"}"
ETCD_DATA_DIR=$(mktemp -d ${TMPDIR}/openshift.local.etcd.XXXX)
VOLUME_DIR=$(mktemp -d ${TMPDIR}/openshift.local.volumes.XXXX)
LOG_DIR="${LOG_DIR:-$(mktemp -d ${TMPDIR}/openshift.local.logs.XXXX)}"
ARTIFACT_DIR="${ARTIFACT_DIR:-$(mktemp -d ${TMPDIR}/openshift.local.artifacts.XXXX)}"
mkdir -p $LOG_DIR
mkdir -p $ARTIFACT_DIR
API_PORT="${API_PORT:-8080}"
API_HOST="${API_HOST:-127.0.0.1}"
KUBELET_PORT="${KUBELET_PORT:-10250}"

CONFIG_FILE="${LOG_DIR}/appConfig.json"
BUILD_CONFIG_FILE="${LOG_DIR}/buildConfig.json"
FIXTURE_DIR="${OS_ROOT}/examples/sample-app"
GO_OUT="${OS_ROOT}/_output/local/go/bin"

# set path so OpenShift is available
export PATH="${GO_OUT}:${PATH}"
pushd "${GO_OUT}" > /dev/null
ln -fs "openshift" "osc"
popd > /dev/null

# teardown
function teardown()
{
  if [ $? -ne 0 ]; then
    echo "[FAIL] !!!!! Test Failed !!!!"
  else
    echo "[INFO] Test Succeeded"
  fi

  echo "[INFO] Dumping container logs to $LOG_DIR"
  for container in $(docker ps -aq); do
    docker logs $container >& $LOG_DIR/container-$container.log
  done

  echo "[INFO] Dumping build log to $LOG_DIR"

  set +e
  BUILD_ID=`osc get -n test builds -o template -t "{{with index .items 0}}{{.metadata.name}}{{end}}"`
  osc build-logs -n test $BUILD_ID > $LOG_DIR/build.log

  curl -L http://localhost:4001/v2/keys/?recursive=true > $ARTIFACT_DIR/etcd_dump.json
  set -e

  echo ""

  set +u
  if [ "$SKIP_TEARDOWN" != "1" ]; then
    set +e
    echo "[INFO] Tearing down test"
    stop_openshift_server
    echo "[INFO] Stopping docker containers"; docker stop $(docker ps -a -q)
    set +u
    if [ "$SKIP_IMAGE_CLEANUP" != "1" ]; then
      echo "[INFO] Removing docker containers"; docker rm $(docker ps -a -q)
    fi
    set -u
    set -e
  fi
  set -u
}

trap teardown EXIT SIGINT

# Setup
stop_openshift_server
echo "[INFO] `openshift version`"
echo "[INFO] Server logs will be at:    $LOG_DIR/openshift.log"
echo "[INFO] Test artifacts will be in: $ARTIFACT_DIR"

# Start All-in-one server and wait for health
echo "[INFO] Starting OpenShift server"
sudo env "PATH=$PATH" openshift start --volume-dir="${VOLUME_DIR}" --etcd-dir="${ETCD_DATA_DIR}" --loglevel=4 &> "${LOG_DIR}/openshift.log" &
OS_PID=$!

wait_for_url "http://localhost:10250/healthz" "[INFO] kubelet: " 1 30
wait_for_url "http://localhost:8080/healthz" "[INFO] apiserver: "

# Deploy private docker registry
echo "[INFO] Deploying private Docker registry"
osc apply -n test -f ${FIXTURE_DIR}/docker-registry-config.json

echo "[INFO] Waiting for Docker registry pod to start"
wait_for_command "osc get -n test pods | grep registrypod | grep -i Running" $((5*TIME_MIN))

echo "[INFO] Waiting for Docker registry service to start"
wait_for_command "osc get -n test services | grep registrypod"

# services can end up on any IP.  Make sure we get the IP we need for the docker registry
DOCKER_REGISTRY_IP=$(osc get -n test -o template --output-version=v1beta1 --template="{{ .portalIP }}" service docker-registry)

echo "[INFO] Probing the docker-registry"
wait_for_url_timed "http://${DOCKER_REGISTRY_IP}:5001" "[INFO] Docker registry says: " $((2*TIME_MIN))

echo "[INFO] Pre-pulling and pushing centos7"
docker pull centos:centos7
echo "[INFO] Pulled centos7"

docker tag -f centos:centos7 ${DOCKER_REGISTRY_IP}:5001/cached/centos:centos7
docker push ${DOCKER_REGISTRY_IP}:5001/cached/centos:centos7
echo "[INFO] Pushed centos7"

# Process template and apply
echo "[INFO] Submitting application template json for processing..."
osc process -n test -f ${FIXTURE_DIR}/application-template-${BUILD_TYPE}build.json > "${CONFIG_FILE}"
# substitute the default IP address with the address where we actually ended up
# TODO: make this be unnecessary by fixing images
# This is no longer needed because the docker registry explicitly requests the 172.30.17.3 ip address.
#sed -i "s,172.30.17.3,${DOCKER_REGISTRY_IP},g" "${CONFIG_FILE}"

echo "[INFO] Applying application config"
osc apply -n test -f "${CONFIG_FILE}"

# Trigger build
echo "[INFO] Invoking generic web hook to trigger new build using curl"
curl -X POST http://localhost:8080/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic?namespace=test && sleep 3

# Wait for build to complete
echo "[INFO] Waiting for build to complete"
wait_for_command "osc get -n test builds | grep -i complete" $((10*TIME_MIN)) "osc get -n test builds | grep -i -e failed -e error"
BUILD_ID=`osc get -n test builds -o template -t "{{with index .items 0}}{{.metadata.name}}{{end}}"`
echo "[INFO] Build ${BUILD_ID} finished"
osc build-logs -n test $BUILD_ID > $LOG_DIR/build.log

# STI builder doesn't currently report a useful success message
#grep -q "Successfully built" $LOG_DIR/build.log

echo "[INFO] Waiting for database pod to start"
wait_for_command "osc get -n test pods -l name=database | grep -i Running" $((30*TIME_SEC))

echo "[INFO] Waiting for database service to start"
wait_for_command "osc get -n test services | grep database" $((20*TIME_SEC))
DB_IP=$(osc get -n test -o template --output-version=v1beta1 --template="{{ .portalIP }}" service database)

echo "[INFO] Waiting for frontend pod to start"
wait_for_command "osc get -n test pods | grep frontend | grep -i Running" $((120*TIME_SEC))

echo "[INFO] Waiting for frontend service to start"
wait_for_command "osc get -n test services | grep frontend" $((20*TIME_SEC))
FRONTEND_IP=$(osc get -n test -o template --output-version=v1beta1 --template="{{ .portalIP }}" service frontend)

echo "[INFO] Waiting for database to start..."
wait_for_url_timed "http://${DB_IP}:5434" "[INFO] Database says: " $((3*TIME_MIN))

echo "[INFO] Waiting for app to start..."
wait_for_url_timed "http://${FRONTEND_IP}:5432" "[INFO] Frontend says: " $((2*TIME_MIN))


if [[ "$ROUTER_TESTS_ENABLED" == "true" ]]; then
    # use the docker bridge ip address until there is a good way to get the address from master
    # this address is considered stable
    apiIP="172.17.42.1"

    echo "{'id':'route', 'kind': 'Route', 'apiVersion': 'v1beta1', 'serviceName': 'frontend', 'host': 'end-to-end'}" > "${ARTIFACT_DIR}/route.json"
    osc create -n test routes -f "${ARTIFACT_DIR}/route.json"

    echo "[INFO] Installing router with master ip of ${apiIP} and starting pod..."
    echo "[INFO] To disable router testing set ROUTER_TESTS_ENABLED=false..."
    "${OS_ROOT}/hack/install-router.sh" "router1" $apiIP
    wait_for_command "osc get pods | grep router | grep -i Running" $((5*TIME_MIN))

    echo "[INFO] Validating routed app response..."
    validate_response "-H Host:end-to-end http://${apiIP}" "Hello from OpenShift" 0.2 50
else
    echo "[INFO] Validating app response..."
    validate_response "http://${FRONTEND_IP}:5432" "Hello from OpenShift"
fi

