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
		echo "You do not have iptables or sudo privileges.	Kubernetes services will not work without iptables access.	See https://github.com/GoogleCloudPlatform/kubernetes/issues/1859.	Try 'sudo hack/test-end-to-end.sh'."
		exit 1
	fi
fi

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

echo "[INFO] Starting end-to-end test"

# Use either the latest release built images, or latest.
if [[ -z "${USE_IMAGES-}" ]]; then
	USE_IMAGES='openshift/origin-${component}:latest'
	if [[ -e "${OS_ROOT}/_output/local/releases/.commit" ]]; then
		COMMIT="$(cat "${OS_ROOT}/_output/local/releases/.commit")"
		USE_IMAGES="openshift/origin-\${component}:${COMMIT}"
	fi
fi

ROUTER_TESTS_ENABLED="${ROUTER_TESTS_ENABLED:-true}"
TEST_ASSETS="${TEST_ASSETS:-false}"

if [[ -z "${BASETMPDIR-}" ]]; then
	TMPDIR="${TMPDIR:-"/tmp"}"
	BASETMPDIR="${TMPDIR}/openshift-e2e"
	sudo rm -rf "${BASETMPDIR}"
	mkdir -p "${BASETMPDIR}"
fi
ETCD_DATA_DIR="${BASETMPDIR}/etcd"
VOLUME_DIR="${BASETMPDIR}/volumes"
CERT_DIR="${BASETMPDIR}/certs"
LOG_DIR="${LOG_DIR:-${BASETMPDIR}/logs}"
ARTIFACT_DIR="${ARTIFACT_DIR:-${BASETMPDIR}/artifacts}"
mkdir -p $LOG_DIR
mkdir -p $ARTIFACT_DIR
API_PORT="${API_PORT:-8443}"
API_SCHEME="${API_SCHEME:-https}"
API_HOST="${API_HOST:-localhost}"
PUBLIC_MASTER_HOST="${PUBLIC_MASTER_HOST:-${API_HOST}}"
KUBELET_SCHEME="${KUBELET_SCHEME:-http}"
KUBELET_PORT="${KUBELET_PORT:-10250}"

# use the docker bridge ip address until there is a good way to get the auto-selected address from master
# this address is considered stable
# Used by the docker-registry and the router pods to call back to the API
CONTAINER_ACCESSIBLE_API_HOST="${CONTAINER_ACCESSIBLE_API_HOST:-172.17.42.1}"

STI_CONFIG_FILE="${LOG_DIR}/stiAppConfig.json"
DOCKER_CONFIG_FILE="${LOG_DIR}/dockerAppConfig.json"
CUSTOM_CONFIG_FILE="${LOG_DIR}/customAppConfig.json"
GO_OUT="${OS_ROOT}/_output/local/go/bin"

# set path so OpenShift is available
export PATH="${GO_OUT}:${PATH}"


function cleanup()
{
	out=$?
	echo
	if [ $out -ne 0 ]; then
		echo "[FAIL] !!!!! Test Failed !!!!"
	else
		echo "[INFO] Test Succeeded"
	fi
	echo

	set +e
	echo "[INFO] Dumping container logs to ${LOG_DIR}"
	for container in $(docker ps -aq); do
		docker logs "$container" >&"${LOG_DIR}/container-$container.log"
	done

	echo "[INFO] Dumping build log to ${LOG_DIR}"

	osc get -n test builds -o template -t '{{ range .items }}{{.metadata.name}}{{ "\n" }}{{end}}' | xargs -r -l osc build-logs -n test >"${LOG_DIR}/stibuild.log"
	osc get -n docker builds -o template -t '{{ range .items }}{{.metadata.name}}{{ "\n" }}{{end}}' | xargs -r -l osc build-logs -n docker >"${LOG_DIR}/dockerbuild.log"
	osc get -n custom builds -o template -t '{{ range .items }}{{.metadata.name}}{{ "\n" }}{{end}}' | xargs -r -l osc build-logs -n custom >"${LOG_DIR}/custombuild.log"
	curl -L http://localhost:4001/v2/keys/?recursive=true > "${ARTIFACT_DIR}/etcd_dump.json"
	echo

	if [[ -z "${SKIP_TEARDOWN-}" ]]; then
		echo "[INFO] Tearing down test"
		pids="$(jobs -pr)"
		echo "[INFO] Children: ${pids}"
		sudo kill ${pids}
		sudo ps f
		set +u
		echo "[INFO] Stopping k8s docker containers"; docker ps | awk '{ print $NF " " $1 }' | grep ^k8s_ | awk '{print $2}'	| xargs -l -r docker stop
		if [[ -z "${SKIP_IMAGE_CLEANUP-}" ]]; then
			echo "[INFO] Removing k8s docker containers"; docker ps -a | awk '{ print $NF " " $1 }' | grep ^k8s_ | awk '{print $2}'	| xargs -l -r docker rm
		fi
		set -u
	fi
	set -e

	# clean up zero byte log files
	# Clean up large log files so they don't end up on jenkins
	find ${ARTIFACT_DIR} -name *.log -size +20M -exec echo Deleting {} because it is too big. \; -exec rm -f {} \;
	find ${LOG_DIR} -name *.log -size +20M -exec echo Deleting {} because it is too big. \; -exec rm -f {} \;
	find ${LOG_DIR} -name *.log -size 0 -exec echo Deleting {} because it is empty. \; -exec rm -f {} \;

	echo "[INFO] Exiting"
	exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

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

# Setup
stop_openshift_server
echo "[INFO] `openshift version`"
echo "[INFO] Server logs will be at:    ${LOG_DIR}/openshift.log"
echo "[INFO] Test artifacts will be in: ${ARTIFACT_DIR}"
echo "[INFO] Volumes dir is:            ${VOLUME_DIR}"
echo "[INFO] Certs dir is:              ${CERT_DIR}"
echo "[INFO] Using images:              ${USE_IMAGES}"

# Start All-in-one server and wait for health
# Specify the scheme and port for the listen address, but let the IP auto-discover.	Set --public-master to localhost, for a stable link to the console.
echo "[INFO] Starting OpenShift server"
sudo env "PATH=${PATH}" OPENSHIFT_ON_PANIC=crash openshift start \
     --listen="${API_SCHEME}://0.0.0.0:${API_PORT}" --public-master="${API_SCHEME}://${PUBLIC_MASTER_HOST}" \
     --hostname="127.0.0.1" --volume-dir="${VOLUME_DIR}" \
     --etcd-dir="${ETCD_DATA_DIR}" --cert-dir="${CERT_DIR}" --loglevel=4 \
     --images="${USE_IMAGES}" \
     &> "${LOG_DIR}/openshift.log" &
OS_PID=$!

if [[ "${API_SCHEME}" == "https" ]]; then
	export CURL_CA_BUNDLE="${CERT_DIR}/master/root.crt"
	export CURL_CERT="${CERT_DIR}/admin/cert.crt"
	export CURL_KEY="${CERT_DIR}/admin/key.key"

	# Generate the certs first
	wait_for_file "${CERT_DIR}/openshift-client/key.key" 0.5 80
	wait_for_file "${CERT_DIR}/admin/key.key" 0.5 80
	wait_for_file "${CURL_CA_BUNDLE}" 0.5 80

	# Read client cert data in to send to containerized components
	sudo chmod -R a+rX "${CERT_DIR}/openshift-client/"

	# Make osc use ${CERT_DIR}/admin/.kubeconfig, and ignore anything in the running user's $HOME dir
	sudo chmod -R a+rwX "${CERT_DIR}/admin/"
	export HOME="${CERT_DIR}/admin"
	export KUBECONFIG="${CERT_DIR}/admin/.kubeconfig"
	echo "[INFO] To debug: export KUBECONFIG=$KUBECONFIG"
fi

wait_for_url "${KUBELET_SCHEME}://${API_HOST}:${KUBELET_PORT}/healthz" "[INFO] kubelet: " 0.5 60
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1beta1/minions/127.0.0.1" "apiserver(minions): " 0.25 80

# Set KUBERNETES_MASTER for osc
export KUBERNETES_MASTER="${API_SCHEME}://${API_HOST}:${API_PORT}"

# create test project so that this shows up in the console
openshift ex new-project test --description="This is an example project to demonstrate OpenShift v3" --admin="anypassword:e2e-user"

echo "The console should be available at ${API_SCHEME}://${PUBLIC_MASTER_HOST}:$(($API_PORT + 1)).	You may need to visit ${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT} first to accept the certificate."
echo "Log in as 'e2e-user' to see the 'test' project."

# install the router
echo "[INFO] Installing the router"
openshift ex router --create --credentials="${KUBECONFIG}" --images="${USE_IMAGES}"

# install the registry. The --mount-host option is provided to reuse local storage.
echo "[INFO] Installing the registry"
# TODO: add --images="${USE_IMAGES}" when the Docker registry is built alongside OpenShift
openshift ex registry --create --credentials="${KUBECONFIG}" --mount-host="/tmp/openshift.local.registry" --images='openshift/origin-${component}:latest'

echo "[INFO] Pre-pulling and pushing centos7"
docker pull centos:centos7
# TODO: remove after this becomes part of the build
docker pull openshift/origin-docker-registry
echo "[INFO] Pulled centos7"

echo "[INFO] Waiting for Docker registry pod to start"
# TODO: simplify when #4702 is fixed upstream
wait_for_command '[[ "$(osc get endpoints docker-registry -t "{{ if .endpoints }}{{ len .endpoints }}{{ else }}0{{ end }}" || echo "0")" != "0" ]]' $((5*TIME_MIN))

# services can end up on any IP.	Make sure we get the IP we need for the docker registry
DOCKER_REGISTRY=$(osc get --output-version=v1beta1 --template="{{ .portalIP }}:{{ .port }}" service docker-registry)

echo "[INFO] Verifying the docker-registry is up at ${DOCKER_REGISTRY}"
wait_for_url_timed "http://${DOCKER_REGISTRY}" "[INFO] Docker registry says: " $((2*TIME_MIN))

docker tag -f centos:centos7 ${DOCKER_REGISTRY}/cached/centos:centos7
docker push ${DOCKER_REGISTRY}/cached/centos:centos7
echo "[INFO] Pushed centos7"

# Process template and create
echo "[INFO] Submitting application template json for processing..."
osc process -n test -f examples/sample-app/application-template-stibuild.json > "${STI_CONFIG_FILE}"
osc process -n docker -f examples/sample-app/application-template-dockerbuild.json > "${DOCKER_CONFIG_FILE}"
osc process -n custom -f examples/sample-app/application-template-custombuild.json > "${CUSTOM_CONFIG_FILE}"

echo "[INFO] Applying STI application config"
osc create -n test -f "${STI_CONFIG_FILE}"

# Trigger build
echo "[INFO] Invoking generic web hook to trigger new sti build using curl"
curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic?namespace=test && sleep 3
wait_for_build "test"
wait_for_app "test"

#echo "[INFO] Applying Docker application config"
#osc create -n docker -f "${DOCKER_CONFIG_FILE}"
#echo "[INFO] Invoking generic web hook to trigger new docker build using curl"
#curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic?namespace=docker && sleep 3
#wait_for_build "docker"
#wait_for_app "docker"

#echo "[INFO] Applying Custom application config"
#osc create -n custom -f "${CUSTOM_CONFIG_FILE}"
#echo "[INFO] Invoking generic web hook to trigger new custom build using curl"
#curl -k -X POST $API_SCHEME://$API_HOST:$API_PORT/osapi/v1beta1/buildConfigHooks/ruby-sample-build/secret101/generic?namespace=custom && sleep 3
#wait_for_build "custom"
#wait_for_app "custom"

# ensure the router is started
# TODO: simplify when #4702 is fixed upstream
wait_for_command '[[ "$(osc get endpoints router -t "{{ if .endpoints }}{{ len .endpoints }}{{ else }}0{{ end }}" || echo "0")" != "0" ]]' $((5*TIME_MIN))

echo "[INFO] Validating routed app response..."
validate_response "-s -k --resolve www.example.com:443:${CONTAINER_ACCESSIBLE_API_HOST} https://www.example.com" "Hello from OpenShift" 0.2 50

# UI e2e tests can be found in assets/test/e2e
if [[ "$TEST_ASSETS" == "true" ]]; then
	echo "[INFO] Running UI e2e tests..."
	pushd ${OS_ROOT}/assets > /dev/null
		grunt test-e2e
	popd > /dev/null
fi
