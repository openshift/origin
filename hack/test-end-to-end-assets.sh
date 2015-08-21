#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

test_privileges

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


TEST_TYPE="openshift-e2e-assets"
TMPDIR="${TMPDIR:-"/tmp"}"
BASETMPDIR="${TMPDIR}/${TEST_TYPE}"

if [[ -d "${BASETMPDIR}" ]]; then
	remove_tmp_dir $TEST_TYPE && mkdir -p "${BASETMPDIR}"
fi

LOG_DIR="${LOG_DIR:-${BASETMPDIR}/logs}"
ARTIFACT_DIR="${ARTIFACT_DIR:-${BASETMPDIR}/artifacts}"
DEFAULT_SERVER_IP=`ifconfig | grep -Ev "(127.0.0.1|172.17.42.1)" | grep "inet " | head -n 1 | sed 's/adr://' | awk '{print $2}'`
API_HOST="${API_HOST:-${DEFAULT_SERVER_IP}}"
setup_env_vars
mkdir -p $LOG_DIR $ARTIFACT_DIR

# use the docker bridge ip address until there is a good way to get the auto-selected address from master
# this address is considered stable
# used as a resolve IP to test routing
CONTAINER_ACCESSIBLE_API_HOST="${CONTAINER_ACCESSIBLE_API_HOST:-172.17.42.1}"

STI_CONFIG_FILE="${LOG_DIR}/stiAppConfig.json"
DOCKER_CONFIG_FILE="${LOG_DIR}/dockerAppConfig.json"
CUSTOM_CONFIG_FILE="${LOG_DIR}/customAppConfig.json"


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
	dump_container_logs

	echo "[INFO] Dumping build log to ${LOG_DIR}"

	oc get -n test builds --output-version=v1beta3 -t '{{ range .items }}{{.metadata.name}}{{ "\n" }}{{end}}' | xargs -r -l oc build-logs -n test >"${LOG_DIR}/stibuild.log"
	oc get -n docker builds --output-version=v1beta3 -t '{{ range .items }}{{.metadata.name}}{{ "\n" }}{{end}}' | xargs -r -l oc build-logs -n docker >"${LOG_DIR}/dockerbuild.log"
	oc get -n custom builds --output-version=v1beta3 -t '{{ range .items }}{{.metadata.name}}{{ "\n" }}{{end}}' | xargs -r -l oc build-logs -n custom >"${LOG_DIR}/custombuild.log"

	echo "[INFO] Dumping etcd contents to ${ARTIFACT_DIR}/etcd_dump.json"
	set_curl_args 0 1
	curl ${clientcert_args} -L "${API_SCHEME}://${API_HOST}:4001/v2/keys/?recursive=true" > "${ARTIFACT_DIR}/etcd_dump.json"
	echo

	if [[ -z "${SKIP_TEARDOWN-}" ]]; then
		echo "[INFO] Switch back to 'default' project with 'admin' user for cleanup"
		oc project ${CLUSTER_ADMIN_CONTEXT}

		echo "[INFO] Deleting test constructs"
		oc delete -n test all --all
		oc delete -n docker all --all
		oc delete -n custom all --all
		oc delete -n cache all --all
		oc delete -n default all --all

		echo "[INFO] Tearing down test"
		kill_all_processes

		set +u
		echo "[INFO] Stopping k8s docker containers"; docker ps | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker stop
		if [[ -z "${SKIP_IMAGE_CLEANUP-}" ]]; then
			echo "[INFO] Removing k8s docker containers"; docker ps -a | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker rm
		fi
		set -u
	fi
	set -e

	delete_large_and_empty_logs

	echo "[INFO] Exiting"
	exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

function wait_for_app() {
	echo "[INFO] Waiting for app in namespace $1"
	echo "[INFO] Waiting for database pod to start"
	wait_for_command "oc get -n $1 pods -l name=database | grep -i Running" $((60*TIME_SEC))

	echo "[INFO] Waiting for database service to start"
	wait_for_command "oc get -n $1 services | grep database" $((20*TIME_SEC))
	DB_IP=$(oc get -n $1 --output-version=v1beta3 --template="{{ .spec.portalIP }}" service database)

	echo "[INFO] Waiting for frontend pod to start"
	wait_for_command "oc get -n $1 pods | grep frontend | grep -i Running" $((120*TIME_SEC))

	echo "[INFO] Waiting for frontend service to start"
	wait_for_command "oc get -n $1 services | grep frontend" $((20*TIME_SEC))
	FRONTEND_IP=$(oc get -n $1 --output-version=v1beta3 --template="{{ .spec.portalIP }}" service frontend)

	echo "[INFO] Waiting for database to start..."
	wait_for_url_timed "http://${DB_IP}:5434" "[INFO] Database says: " $((3*TIME_MIN))

	echo "[INFO] Waiting for app to start..."
	wait_for_url_timed "http://${FRONTEND_IP}:5432" "[INFO] Frontend says: " $((2*TIME_MIN))

	echo "[INFO] Testing app"
	wait_for_command '[[ "$(curl -s -X POST http://${FRONTEND_IP}:5432/keys/foo -d value=1337)" = "Key created" ]]'
	wait_for_command '[[ "$(curl -s http://${FRONTEND_IP}:5432/keys/foo)" = "1337" ]]'
}

# Wait for builds to complete
# $1 namespace
function wait_for_build() {
	echo "[INFO] Waiting for $1 namespace build to complete"
	wait_for_command "oc get -n $1 builds | grep -i complete" $((10*TIME_MIN)) "oc get -n $1 builds | grep -i -e failed -e error"
	BUILD_ID=`oc get -n $1 builds --output-version=v1beta3 -t "{{with index .items 0}}{{.metadata.name}}{{end}}"`
	echo "[INFO] Build ${BUILD_ID} finished"
  # TODO: fix
  set +e
	oc build-logs -n $1 $BUILD_ID > $LOG_DIR/$1build.log
  set -e
}

# Setup
echo "[INFO] `openshift version`"
echo "[INFO] Server logs will be at:    ${LOG_DIR}/openshift.log"
echo "[INFO] Test artifacts will be in: ${ARTIFACT_DIR}"
echo "[INFO] Volumes dir is:            ${VOLUME_DIR}"
echo "[INFO] Config dir is:             ${SERVER_CONFIG_DIR}"
echo "[INFO] Using images:              ${USE_IMAGES}"

# Start All-in-one server and wait for health
echo "[INFO] Create certificates for the OpenShift server"
# find the same IP that openshift start will bind to.  This allows access from pods that have to talk back to master
ALL_IP_ADDRESSES=`ifconfig | grep "inet " | sed 's/adr://' | awk '{print $2}'`
SERVER_HOSTNAME_LIST="${PUBLIC_MASTER_HOST},localhost"
while read -r IP_ADDRESS
do
	SERVER_HOSTNAME_LIST="${SERVER_HOSTNAME_LIST},${IP_ADDRESS}"
done <<< "${ALL_IP_ADDRESSES}"

configure_os_server

export HOME="${FAKE_HOME_DIR}"
# This directory must exist so Docker can store credentials in $HOME/.dockercfg
mkdir -p ${FAKE_HOME_DIR}

export KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
CLUSTER_ADMIN_CONTEXT=$(oc config view --flatten -o template -t '{{index . "current-context"}}')

if [[ "${API_SCHEME}" == "https" ]]; then
	export CURL_CA_BUNDLE="${MASTER_CONFIG_DIR}/ca.crt"
	export CURL_CERT="${MASTER_CONFIG_DIR}/admin.crt"
	export CURL_KEY="${MASTER_CONFIG_DIR}/admin.key"

	# Make oc use ${MASTER_CONFIG_DIR}/admin.kubeconfig, and ignore anything in the running user's $HOME dir
	sudo chmod -R a+rwX "${KUBECONFIG}"
	echo "[INFO] To debug: export KUBECONFIG=$KUBECONFIG"
fi

start_os_server

# install the registry. The --mount-host option is provided to reuse local storage.
echo "[INFO] Installing the registry"
openshift admin registry --create --credentials="${MASTER_CONFIG_DIR}/openshift-registry.kubeconfig" --images="${USE_IMAGES}"

# pre-load some image streams and templates
oc create -f examples/image-streams/image-streams-centos7.json --namespace=openshift
oc create -f examples/sample-app/application-template-stibuild.json --namespace=openshift

# create a test project so that this shows up in the console
openshift admin new-project "test" --description="This is an example project to demonstrate OpenShift v3" --admin="e2e-user"

echo "[INFO] Running UI e2e tests..."
pushd ${OS_ROOT}/assets > /dev/null
	grunt test-e2e-chrome
popd > /dev/null
