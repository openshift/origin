#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app

# Default to use STI builder if no argument specified
BUILD_TYPE=${1:-sti}

iptables --list > /dev/null 2>&1
if [ $? -ne 0 ]; then
  echo "You do not have iptables privileges.  Kubernetes services will not work without iptables access.  See https://github.com/GoogleCloudPlatform/kubernetes/issues/1859.  Try 'sudo hack/test-end-to-end.sh'."
  exit 1
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
NAMESPACE="${NAMESPACE:-test}"

CONFIG_FILE="${LOG_DIR}/appConfig.json"
BUILD_CONFIG_FILE="${LOG_DIR}/buildConfig.json"
FIXTURE_DIR="${OS_ROOT}/examples/sample-app"
GO_OUT="${OS_ROOT}/_output/local/go/bin"
openshift="${GO_OUT}/openshift"
cli="${GO_OUT}/openshift --loglevel=0 cli"

# setup()
function setup()
{
  stop_openshift_server
  echo "[INFO] `$openshift version`"
  echo "[INFO] Server logs will be at:    $LOG_DIR/openshift.log"
  echo "[INFO] Test artifacts will be in: $ARTIFACT_DIR"
}

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

  curl -L http://localhost:4001/v2/keys/?recursive=true > $ARTIFACT_DIR/etcd_dump.json

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

setup

# Start All-in-one server and wait for health
echo "[INFO] Starting OpenShift server"
start_openshift_server ${VOLUME_DIR} ${ETCD_DATA_DIR} ${LOG_DIR}

wait_for_url "http://localhost:10250/healthz" "[INFO] kubelet: " 1 30
wait_for_url "http://localhost:8080/healthz" "[INFO] apiserver: "

# Deploy private docker registry
echo "[INFO] Deploying private Docker registry"
${cli} apply -n ${NAMESPACE} -f ${FIXTURE_DIR}/docker-registry-config.json

echo "[INFO] Waiting for Docker registry pod to start"
wait_for_command "${cli} get -n ${NAMESPACE} pods | grep registrypod | grep -i Running" $((5*TIME_MIN))

echo "[INFO] Waiting for Docker registry service to start"
wait_for_command "${cli} get -n ${NAMESPACE} services | grep registrypod"
# services can end up on any IP.  Make sure we get the IP we need for the docker registry
DOCKER_REGISTRY_IP=`${cli} get -n ${NAMESPACE} -o yaml service docker-registry | grep "portalIP" | awk '{print $2}'`

echo "[INFO] Probing the docker-registry"
wait_for_url_timed "http://${DOCKER_REGISTRY_IP}:5001" "[INFO] Docker registry says: " $((2*TIME_MIN))

echo "[INFO] Pre-pulling and pushing centos7"
STARTTIME=$(date +%s)
docker pull centos:centos7
ENDTIME=$(date +%s)
echo "[INFO] Pulled centos7: $(($ENDTIME - $STARTTIME)) seconds"

docker tag centos:centos7 ${DOCKER_REGISTRY_IP}:5001/cached/centos:centos7
STARTTIME=$(date +%s)
docker push ${DOCKER_REGISTRY_IP}:5001/cached/centos:centos7
ENDTIME=$(date +%s)
echo "[INFO] Pushed centos7: $(($ENDTIME - $STARTTIME)) seconds"


# Process template and apply
echo "[INFO] Submitting application template json for processing..."
${cli} process -n ${NAMESPACE} -f ${FIXTURE_DIR}/application-template-${BUILD_TYPE}build.json > $CONFIG_FILE
# substitute the default IP address with the address where we actually ended up
sed -i "s,172.121.17.3,${DOCKER_REGISTRY_IP},g" $CONFIG_FILE

echo "[INFO] Applying application config"
${cli} apply -n ${NAMESPACE} -f $CONFIG_FILE

# Trigger build
echo "[INFO] Invoking generic web hook to trigger new build using curl"
curl -X POST http://localhost:8080/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic?namespace=${NAMESPACE} && sleep 3

# Wait for build to complete
echo "[INFO] Waiting for build to complete"
wait_for_command "${cli} get -n ${NAMESPACE} builds | grep -i complete" $((10*TIME_MIN)) "${cli} get -n ${NAMESPACE} builds | grep -i -e failed -e error"
BUILD_ID=`${cli} get -n ${NAMESPACE} builds -o template -t "{{with index .items 0}}{{.metadata.name}}{{end}}"`

echo "[INFO] Build '${BUILD_ID}' details:"
${cli} describe -n ${NAMESPACE} build ${BUILD_ID}
${cli} build-logs -n ${NAMESPACE} $BUILD_ID > $LOG_DIR/build.log

# STI builder doesn't currently report a useful success message
#grep -q "Successfully built" $LOG_DIR/build.log

echo "[INFO] Waiting for database pod to start"
wait_for_command "${cli} get -n ${NAMESPACE} pods -l name=database | grep -i Running" $((30*TIME_SEC))

echo "[INFO] Waiting for database service to start"
wait_for_command "${cli} get -n ${NAMESPACE} services | grep database" $((20*TIME_SEC))
DB_IP=`${cli} get -n ${NAMESPACE} -o yaml service database | grep "portalIP" | awk '{print $2}'`

echo "[INFO] Waiting for frontend pod to start"
wait_for_command "${cli} get -n ${NAMESPACE} pods | grep frontend | grep -i Running" $((120*TIME_SEC))

echo "[INFO] Waiting for frontend service to start"
wait_for_command "${cli} get -n ${NAMESPACE} services | grep frontend" $((20*TIME_SEC))
FRONTEND_IP=`${cli} get -n ${NAMESPACE} -o yaml service frontend | grep "portalIP" | awk '{print $2}'`

echo "[INFO] Waiting for database to start..."
wait_for_url_timed "http://${DB_IP}:5434" "[INFO] Database says: " $((3*TIME_MIN))

echo "[INFO] Waiting for app to start..."
wait_for_url_timed "http://${FRONTEND_IP}:5432" "[INFO] Frontend says: " $((2*TIME_MIN))


if [[ "$ROUTER_TESTS_ENABLED" == "true" ]]; then
    # use the docker bridge ip address until there is a good way to get the address from master
    # this address is considered stable
    apiIP="172.17.42.1"

    echo "[INFO] Installing router with master ip of ${apiIP} and starting pod..."
    echo "[INFO] To disable router testing set ROUTER_TESTS_ENABLED=false..."
    "${OS_ROOT}/hack/install-router.sh" "router1" $apiIP $openshift
    wait_for_command "${cli} get pods | grep router | grep -i Running" $((5*TIME_MIN))

    echo "[INFO] Validating routed app response doesn't exist"
    validate_response "-H Host:end-to-end --connect-timeout 10 http://${apiIP}" "503 Service Unavailable" 2 $((2*TIME_MIN))

    echo "{'id':'route', 'kind': 'Route', 'apiVersion': 'v1beta1', 'serviceName': 'frontend', 'host': 'end-to-end'}" > "${ARTIFACT_DIR}/route.json"
    ${cli} create -n ${NAMESPACE} routes -f "${ARTIFACT_DIR}/route.json"

    echo "[INFO] Validating routed app response..."
    validate_response "-H Host:end-to-end http://${apiIP}" "Hello from OpenShift"
else
    echo "[INFO] Validating app response..."
    validate_response "http://${FRONTEND_IP}:5432" "Hello from OpenShift"
fi

