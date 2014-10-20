#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app

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
CONFIG_FILE=${LOG_DIR}/appConfig.json
FIXTURE_DIR=${HACKDIR}/../examples/sample-app
GO_OUT=${HACKDIR}/../_output/go/bin
openshift=$GO_OUT/openshift

# setup()
function setup()
{
  cleanup_openshift_server
  echo "[INFO] `$openshift version`"
}

# teardown
function teardown()
{
  if [ $? -ne 0 ]; then
    echo "[FAIL] !!!!! Test Failed !!!!"
	echo "[INFO] Server logs: $LOG_DIR/openshift.log"
	set +u
    if [ ! -z $BUILD_ID ]; then
	 $openshift kube buildLogs --id=$BUILD_ID > $LOG_DIR/build.log && echo "[INFO] Build logs: $LOG_DIR/build.log"
	fi
	set -u
  fi
  set +u
  if [ "$SKIP_TEARDOWN" != "1" ]; then
    set +e
    echo "[INFO] Tearing down test"
    cleanup_openshift_server
    echo "[INFO] Stopping docker containers"; docker stop $(docker ps -a -q)
    echo "[INFO] Removing docker containers"; docker rm $(docker ps -a -q)
    set -e
  fi
  set -u
}

trap teardown EXIT

setup

# Start All-in-one server and wait for health
echo "[INFO] Starting OpenShift server"
$openshift start --volume-dir=${VOLUME_DIR} --etcd-dir=${ETCD_DATA_DIR} &> ${LOG_DIR}/openshift.log &

wait_for_url "http://localhost:10250/healthz" "[INFO] kubelet: " 1 30
wait_for_url "http://localhost:8080/healthz" "[INFO] apiserver: "

# Deploy private docker registry
echo "[INFO] Deploying private Docker registry"
$openshift kube apply -c ${FIXTURE_DIR}/registry-config.json

echo "[INFO] Waiting for Docker registry pod to start"
wait_for_command "$openshift kube list pods | grep registryPod | grep Running" $((5*TIME_MIN))

echo "[INFO] Waiting for Docker registry service to start"
wait_for_command "$openshift kube list services | grep registryPod"

# Define a build configuration
echo "[INFO] Create a build config"
wait_for_command "$openshift kube create buildConfigs -c ${FIXTURE_DIR}/buildcfg/buildcfg.json"

# Trigger build
echo "[INFO] Simulating github hook to trigger new build using curl"
curl -s -A "GitHub-Hookshot/github" -H "Content-Type:application/json" -H "X-Github-Event:push" -d @${FIXTURE_DIR}/buildinvoke/pushevent.json http://localhost:8080/osapi/v1beta1/buildConfigHooks/build100/secret101/github

# Wait for build to complete
echo "[INFO] Waiting for build to complete"
BUILD_ID=`$openshift kube list builds --template="{{with index .Items 0}}{{.ID}}{{end}}"`
wait_for_command "$openshift kube get builds/$BUILD_ID | grep complete" $((20*TIME_MIN)) "$openshift kube get builds/$BUILD_ID | grep failed"

# Process template and apply
echo "[INFO] Submitting application template json for processing..."
$openshift kube process -c ${FIXTURE_DIR}/template/template.json > $CONFIG_FILE

echo "[INFO] Applying application config"
$openshift kube  apply -c $CONFIG_FILE

echo "[INFO] Waiting for frontend pod to start"
wait_for_command "$openshift kube list pods | grep frontend | grep Running" $((30*TIME_SEC))

echo "[INFO] Waiting for frontend service to start"
wait_for_command "$openshift kube list services | grep frontend" $((20*TIME_SEC))

echo "[INFO] Waiting for app to start..."
wait_for_url_timed "http://localhost:5432" "[INFO] Frontend says: " $((2*TIME_MIN))
