#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app

iptables --list > /dev/null 2>&1
if [ $? -ne 0 ]; then
  echo "You do not have iptables privileges.  Kubernetes services will not work without iptables access.  See https://github.com/GoogleCloudPlatform/kubernetes/issues/1859.  Try 'sudo hack/test-end-to-end.sh'."
  exit 1
fi

set -o errexit
set -o nounset
set -o pipefail

echo "[INFO] Starting end-to-end test"

HACKDIR=$(CDPATH="" cd $(dirname $0); pwd)
source ${HACKDIR}/util.sh

TMPDIR=${TMPDIR:-"/tmp"}
ETCD_DATA_DIR=$(mktemp -d ${TMPDIR}/openshift.local.etcd.XXXX)
VOLUME_DIR=$(mktemp -d ${TMPDIR}/openshift.local.volumes.XXXX)
LOG_DIR=${LOG_DIR:-$(mktemp -d ${TMPDIR}/openshift.local.logs.XXXX)}
API_PORT=${API_PORT:-8080}
API_HOST=${API_HOST:-127.0.0.1}
KUBELET_PORT=${KUBELET_PORT:-10250}

CONFIG_FILE=${LOG_DIR}/appConfig.json
FIXTURE_DIR=${HACKDIR}/../examples/sample-app
GO_OUT=${HACKDIR}/../_output/go/bin
openshift=$GO_OUT/openshift

# setup()
function setup()
{
  stop_openshift_server
  echo "[INFO] `$openshift version`"
}

# teardown
function teardown()
{
  if [ $? -ne 0 ]; then
    echo "[FAIL] !!!!! Test Failed !!!!"
  echo "[INFO] Server logs: $LOG_DIR/openshift.log"
  cat $LOG_DIR/openshift.log | grep -v "failed to find a fit"
  set +u
  if [ ! -z $BUILD_ID ]; then
    $openshift kube buildLogs --id=$BUILD_ID > $LOG_DIR/build.log && echo "[INFO] Build logs: $LOG_DIR/build.log"
    cat $LOG_DIR/build.log
  fi
  set -u
  fi
  set +u
  if [ "$SKIP_TEARDOWN" != "1" ]; then
    set +e
    echo "[INFO] Tearing down test"
    stop_openshift_server
    echo "[INFO] Stopping docker containers"; docker stop $(docker ps -a -q)
    echo "[INFO] Removing docker containers"; docker rm $(docker ps -a -q)
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
$openshift kube apply -c ${FIXTURE_DIR}/docker-registry-config.json

echo "[INFO] Waiting for Docker registry pod to start"
wait_for_command "$openshift kube list pods | grep registrypod | grep Running" $((5*TIME_MIN))

echo "[INFO] Waiting for Docker registry service to start"
wait_for_command "$openshift kube list services | grep registrypod"
# services can end up on any IP.  Make sure we get the IP we need for the docker registry
DOCKER_REGISTRY_IP=`$openshift kube get --yaml services/docker-registry | grep "portalIP" | awk '{print $2}'`

echo "[INFO] Probing the docker-registry"
wait_for_url_timed "http://${DOCKER_REGISTRY_IP}:5001" "[INFO] Docker registry says: " $((2*TIME_MIN))

# Process template and apply
echo "[INFO] Submitting application template json for processing..."
$openshift kube process -c ${FIXTURE_DIR}/application-template.json > $CONFIG_FILE
# substitute the default IP address with the address where we actually ended up
sed -i "s,172.121.17.1,${DOCKER_REGISTRY_IP},g" $CONFIG_FILE

echo "[INFO] Applying application config"
$openshift kube apply -c $CONFIG_FILE

# Trigger build
echo "[INFO] Simulating github hook to trigger new build using curl"
curl -s -A "GitHub-Hookshot/github" -H "Content-Type:application/json" -H "X-Github-Event:push" -d @${FIXTURE_DIR}/github-webhook-example.json http://localhost:8080/osapi/v1beta1/buildConfigHooks/build100/secret101/github

# Wait for build to complete
echo "[INFO] Waiting for build to complete"
BUILD_ID=`$openshift kube list builds --template="{{with index .Items 0}}{{.ID}}{{end}}"`
wait_for_command "$openshift kube get builds/$BUILD_ID | grep complete" $((30*TIME_MIN)) "$openshift kube get builds/$BUILD_ID | grep failed"

echo "[INFO] Waiting for frontend pod to start"
wait_for_command "$openshift kube list pods | grep frontend | grep Running" $((120*TIME_SEC))

echo "[INFO] Waiting for frontend service to start"
wait_for_command "$openshift kube list services | grep frontend" $((20*TIME_SEC))
FRONTEND_IP=`$openshift kube get --yaml services/frontend | grep "portalIP" | awk '{print $2}'`

echo "[INFO] Waiting for app to start..."
wait_for_url_timed "http://${FRONTEND_IP}:5432" "[INFO] Frontend says: " $((2*TIME_MIN))
