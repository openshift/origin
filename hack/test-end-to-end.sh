#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app

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

echo "[INFO] Starting end-to-end test"

USE_LOCAL_IMAGES="${USE_LOCAL_IMAGES:-true}"
ROUTER_TESTS_ENABLED="${ROUTER_TESTS_ENABLED:-true}"

TMPDIR="${TMPDIR:-"/tmp"}"
ETCD_DATA_DIR=$(mktemp -d ${TMPDIR}/openshift.local.etcd.XXXX)
VOLUME_DIR=$(mktemp -d ${TMPDIR}/openshift.local.volumes.XXXX)
CERT_DIR=$(mktemp -d ${TMPDIR}/openshift.local.certificates.XXXX)
LOG_DIR="${LOG_DIR:-$(mktemp -d ${TMPDIR}/openshift.local.logs.XXXX)}"
ARTIFACT_DIR="${ARTIFACT_DIR:-$(mktemp -d ${TMPDIR}/openshift.local.artifacts.XXXX)}"
mkdir -p $LOG_DIR
mkdir -p $ARTIFACT_DIR
API_PORT="${API_PORT:-8443}"
API_SCHEME="${API_SCHEME:-https}"
API_HOST="${API_HOST:-localhost}"
KUBELET_SCHEME="${KUBELET_SCHEME:-http}"
KUBELET_PORT="${KUBELET_PORT:-10250}"

# use the docker bridge ip address until there is a good way to get the auto-selected address from master
# this address is considered stable
# Used by the docker-registry and the router pods to call back to the API
CONTAINER_ACCESSIBLE_API_HOST="172.17.42.1"

STI_CONFIG_FILE="${LOG_DIR}/stiAppConfig.json"
DOCKER_CONFIG_FILE="${LOG_DIR}/dockerAppConfig.json"
CUSTOM_CONFIG_FILE="${LOG_DIR}/customAppConfig.json"
GO_OUT="${OS_ROOT}/_output/local/go/bin"

# set path so OpenShift is available
export PATH="${GO_OUT}:${PATH}"

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
  osc get -n test builds -o template -t '{{ range .items }}{{.metadata.name}}{{ "\n" }}{{end}}' | xargs -r -l osc build-logs -n test >$LOG_DIR/stibuild.log
  osc get -n docker builds -o template -t '{{ range .items }}{{.metadata.name}}{{ "\n" }}{{end}}' | xargs -r -l osc build-logs -n docker >$LOG_DIR/dockerbuild.log
  osc get -n custom builds -o template -t '{{ range .items }}{{.metadata.name}}{{ "\n" }}{{end}}' | xargs -r -l osc build-logs -n custom >$LOG_DIR/custombuild.log

  curl -L http://localhost:4001/v2/keys/?recursive=true > $ARTIFACT_DIR/etcd_dump.json
  set -e

  echo ""

  set +u
  if [ "$SKIP_TEARDOWN" != "1" ]; then
    set +e
    echo "[INFO] Tearing down test"
    stop_openshift_server
    echo "[INFO] Stopping docker containers"; docker ps -aq | xargs -l -r docker stop
    set +u
    if [ "$SKIP_IMAGE_CLEANUP" != "1" ]; then
      echo "[INFO] Removing docker containers"; docker ps -aq | xargs -l -r docker rm
    fi
    set -u
    set -e
  fi
  set -u
}

function wait_for_app() {
  echo "[INFO] Waiting for app in namespace $1"
  echo "[INFO] Waiting for database pod to start"
  wait_for_command "osc get -n $1 pods -l name=database | grep -i Running" $((30*TIME_SEC))

  echo "[INFO] Waiting for database service to start"
  wait_for_command "osc get -n $1 services | grep database" $((20*TIME_SEC))
  DB_IP=$(osc get -n $1 -o template --output-version=v1beta1 --template="{{ .portalIP }}" service database)

  echo "[INFO] Waiting for frontend pod to start"
  wait_for_command "osc get -n $1 pods | grep frontend | grep -i Running" $((120*TIME_SEC))

  echo "[INFO] Waiting for frontend service to start"
  wait_for_command "osc get -n $1 services | grep frontend" $((20*TIME_SEC))
  FRONTEND_IP=$(osc get -n $1 -o template --output-version=v1beta1 --template="{{ .portalIP }}" service frontend)

  echo "[INFO] Waiting for database to start..."
  wait_for_url_timed "http://${DB_IP}:5434" "[INFO] Database says: " $((3*TIME_MIN))

  echo "[INFO] Waiting for app to start..."
  wait_for_url_timed "http://${FRONTEND_IP}:5432" "[INFO] Frontend says: " $((2*TIME_MIN))  
}

# Wait for builds to complete
# $1 namespace
function wait_for_build() {
  echo "[INFO] Waiting for $1 namespace build to complete"
  wait_for_command "osc get -n $1 builds | grep -i complete" $((10*TIME_MIN)) "osc get -n $1 builds | grep -i -e failed -e error"
  BUILD_ID=`osc get -n $1 builds -o template -t "{{with index .items 0}}{{.metadata.name}}{{end}}"`
  echo "[INFO] Build ${BUILD_ID} finished"
  osc build-logs -n $1 $BUILD_ID > $LOG_DIR/$1build.log
}

trap teardown EXIT SIGINT

# Setup
stop_openshift_server
echo "[INFO] `openshift version`"
echo "[INFO] Server logs will be at:    $LOG_DIR/openshift.log"
echo "[INFO] Test artifacts will be in: $ARTIFACT_DIR"
echo "[INFO] Volumes dir is:            $VOLUME_DIR"
echo "[INFO] Certs dir is:              $CERT_DIR"

# Start All-in-one server and wait for health
# Specify the scheme and port for the master, but let the IP auto-discover
echo "[INFO] Starting OpenShift server"
sudo env "PATH=$PATH" openshift start --listen=$API_SCHEME://0.0.0.0:$API_PORT --volume-dir="${VOLUME_DIR}" --etcd-dir="${ETCD_DATA_DIR}" --cert-dir="${CERT_DIR}" --loglevel=4 &> "${LOG_DIR}/openshift.log" &
OS_PID=$!

if [[ "$API_SCHEME" == "https" ]]; then
	export CURL_CA_BUNDLE="$CERT_DIR/master/root.crt"
fi

wait_for_url "$KUBELET_SCHEME://$API_HOST:$KUBELET_PORT/healthz" "[INFO] kubelet: " 1 30
wait_for_url "$API_SCHEME://$API_HOST:$API_PORT/healthz" "[INFO] apiserver: "

# Set KUBERNETES_MASTER for osc
export KUBERNETES_MASTER=$API_SCHEME://$API_HOST:$API_PORT
if [[ "$API_SCHEME" == "https" ]]; then
	# Read client cert data in to send to containerized components
	sudo chmod 644 $CERT_DIR/openshift-client/*
	OPENSHIFT_CA_DATA=$(<$CERT_DIR/openshift-client/root.crt)
	OPENSHIFT_CERT_DATA=$(<$CERT_DIR/openshift-client/cert.crt)
	OPENSHIFT_KEY_DATA=$(<$CERT_DIR/openshift-client/key.key)

	# Make osc use $CERT_DIR/admin/.kubeconfig, and ignore anything in the running user's $HOME dir
	sudo chmod 644 $CERT_DIR/admin/*
	export HOME=$CERT_DIR/admin
	export KUBECONFIG=$CERT_DIR/admin/.kubeconfig
  echo "[INFO] To debug: export KUBECONFIG=$KUBECONFIG"
else
	OPENSHIFT_CA_DATA=""
	OPENSHIFT_CERT_DATA=""
	OPENSHIFT_KEY_DATA=""
fi

# Deploy private docker registry
echo "[INFO] Submitting docker-registry template file for processing"
osc process -n test -f examples/sample-app/docker-registry-template.json -v "OPENSHIFT_MASTER=$API_SCHEME://${CONTAINER_ACCESSIBLE_API_HOST}:$API_PORT,OPENSHIFT_CA_DATA=${OPENSHIFT_CA_DATA},OPENSHIFT_CERT_DATA=${OPENSHIFT_CERT_DATA},OPENSHIFT_KEY_DATA=${OPENSHIFT_KEY_DATA}" > "$ARTIFACT_DIR/docker-registry-config.json"

echo "[INFO] Deploying private Docker registry from $ARTIFACT_DIR/docker-registry-config.json"
osc apply -n test -f ${ARTIFACT_DIR}/docker-registry-config.json

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
osc process -n test -f examples/sample-app/application-template-stibuild.json > "${STI_CONFIG_FILE}"
osc process -n docker -f examples/sample-app/application-template-dockerbuild.json > "${DOCKER_CONFIG_FILE}"
osc process -n custom -f examples/sample-app/application-template-custombuild.json > "${CUSTOM_CONFIG_FILE}"

# substitute the default IP address with the address where we actually ended up
# TODO: make this be unnecessary by fixing images
# This is no longer needed because the docker registry explicitly requests the 172.30.17.3 ip address.
#sed -i "s,172.30.17.3,${DOCKER_REGISTRY_IP},g" "${CONFIG_FILE}"

echo "[INFO] Applying STI application config"
osc apply -n test -f "${STI_CONFIG_FILE}"

# Trigger build
echo "[INFO] Invoking generic web hook to trigger new sti build using curl"
curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic?namespace=test && sleep 3
wait_for_build "test"
wait_for_app "test"

#echo "[INFO] Applying Docker application config"
#osc apply -n docker -f "${DOCKER_CONFIG_FILE}"
#echo "[INFO] Invoking generic web hook to trigger new docker build using curl"
#curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic?namespace=docker && sleep 3
#wait_for_build "docker"
#wait_for_app "docker"

#echo "[INFO] Applying Custom application config"
#osc apply -n custom -f "${CUSTOM_CONFIG_FILE}"
#echo "[INFO] Invoking generic web hook to trigger new custom build using curl"
#curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic?namespace=custom && sleep 3
#wait_for_build "custom"
#wait_for_app "custom"

if [[ "$ROUTER_TESTS_ENABLED" == "true" ]]; then
    echo "{'id':'route', 'kind': 'Route', 'apiVersion': 'v1beta1', 'serviceName': 'frontend', 'host': 'end-to-end'}" > "${ARTIFACT_DIR}/route.json"
    osc create -n test routes -f "${ARTIFACT_DIR}/route.json"

    echo "[INFO] Installing router with master ip of ${CONTAINER_ACCESSIBLE_API_HOST} and starting pod..."
    echo "[INFO] To disable router testing set ROUTER_TESTS_ENABLED=false..."

    # update the template file
    cp ${OS_ROOT}/images/router/haproxy/template.json $ARTIFACT_DIR/router-template.json
    sed -i s/ROUTER_ID/router1/g $ARTIFACT_DIR/router-template.json

    echo "[INFO] Submitting router pod template file for processing"
    osc process -n test -f $ARTIFACT_DIR/router-template.json -v "OPENSHIFT_MASTER=$API_SCHEME://${CONTAINER_ACCESSIBLE_API_HOST}:$API_PORT,OPENSHIFT_CA_DATA=${OPENSHIFT_CA_DATA},OPENSHIFT_CERT_DATA=${OPENSHIFT_CERT_DATA},OPENSHIFT_KEY_DATA=${OPENSHIFT_KEY_DATA}" > "$ARTIFACT_DIR/router.json"

    echo "[INFO] Applying router pod config"
    osc apply -n test -f "$ARTIFACT_DIR/router.json"

    wait_for_command "osc get -n test pods | grep router | grep -i Running" $((5*TIME_MIN))

    echo "[INFO] Validating routed app response..."
    validate_response "-H Host:end-to-end http://${CONTAINER_ACCESSIBLE_API_HOST}" "Hello from OpenShift" 0.2 50
else
    echo "[INFO] Validating app response..."
    validate_response "http://${FRONTEND_IP}:5432" "Hello from OpenShift"
fi

