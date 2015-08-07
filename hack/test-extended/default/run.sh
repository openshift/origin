#!/bin/bash
#
# This scripts starts the OpenShift server with a default configuration.
# The OpenShift Docker registry and router are installed.
# It will start all 'default_*_test.go' test cases.

set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../../..
cd "${OS_ROOT}"

source ${OS_ROOT}/hack/util.sh
source ${OS_ROOT}/hack/common.sh


cleanup() {
    stop_openshift_server
    rm -rf ${ETCD_DIR-}

	echo "[INFO] Stopping k8s docker containers"; docker ps | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker stop
	if [[ -z "${SKIP_IMAGE_CLEANUP-}" ]]; then
		echo "[INFO] Removing k8s docker containers"; docker ps -a | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker rm
	fi

    echo "[INFO] Cleanup complete"
}

test_privileges

echo "[INFO] Starting 'default' extended tests"

TIME_SEC=1000
TIME_MIN=$((60 * $TIME_SEC))

TEST_TYPE="openshift-extended-tests"
TMPDIR="${TMPDIR:-"/tmp"}"
BASETMPDIR="${TMPDIR}/${TEST_TYPE}"

if [[ -d "${BASETMPDIR}" ]]; then
	remove_tmp_dir $TEST_TYPE && mkdir -p "${BASETMPDIR}"
fi

# Use either the latest release built images, or latest.
if [[ -z "${USE_IMAGES-}" ]]; then
	USE_IMAGES='openshift/origin-${component}:latest'
	if [[ -e "${OS_ROOT}/_output/local/releases/.commit" ]]; then
		COMMIT="$(cat "${OS_ROOT}/_output/local/releases/.commit")"
		USE_IMAGES="openshift/origin-\${component}:${COMMIT}"
	fi
fi

LOG_DIR="${LOG_DIR:-${BASETMPDIR}/logs}"
ARTIFACT_DIR="${ARTIFACT_DIR:-${BASETMPDIR}/artifacts}"
DEFAULT_SERVER_IP=`ifconfig | grep -Ev "(127.0.0.1|172.17.42.1)" | grep "inet " | head -n 1 | sed 's/adr://' | awk '{print $2}'`
API_HOST="${API_HOST:-${DEFAULT_SERVER_IP}}"
setup_env_vars
mkdir -p $LOG_DIR $ARTIFACT_DIR

ln -svf `readlink -f $(which openshift)` `dirname $(which openshift)`/atomic-enterprise

# use the docker bridge ip address until there is a good way to get the auto-selected address from master
# this address is considered stable
# used as a resolve IP to test routing
CONTAINER_ACCESSIBLE_API_HOST="${CONTAINER_ACCESSIBLE_API_HOST:-172.17.42.1}"

STI_CONFIG_FILE="${LOG_DIR}/stiAppConfig.json"
DOCKER_CONFIG_FILE="${LOG_DIR}/dockerAppConfig.json"
CUSTOM_CONFIG_FILE="${LOG_DIR}/customAppConfig.json"

trap "exit" INT TERM
trap "cleanup" EXIT

# Setup
stop_openshift_server
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

export ADMIN_KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
CLUSTER_ADMIN_CONTEXT=$(oc config view --flatten -o template -t '{{index . "current-context"}}')

if [[ "${API_SCHEME}" == "https" ]]; then
	export CURL_CA_BUNDLE="${MASTER_CONFIG_DIR}/ca.crt"
	export CURL_CERT="${MASTER_CONFIG_DIR}/admin.crt"
	export CURL_KEY="${MASTER_CONFIG_DIR}/admin.key"

	# Make oc use ${MASTER_CONFIG_DIR}/admin.kubeconfig, and ignore anything in the running user's $HOME dir
	sudo chmod -R a+rwX "${ADMIN_KUBECONFIG}"
	echo "[INFO] To debug: export ADMIN_KUBECONFIG=$ADMIN_KUBECONFIG"
fi

start_os_server

# install the router
echo "[INFO] Installing the router"
echo '{"kind":"ServiceAccount","apiVersion":"v1","metadata":{"name":"router"}}' | oc create -f - --config="${ADMIN_KUBECONFIG}"
oc get scc privileged -o json --config="${ADMIN_KUBECONFIG}" | sed '/\"users\"/a \"system:serviceaccount:default:router\",' | oc replace scc privileged -f - --config="${ADMIN_KUBECONFIG}"
openshift admin router --create --credentials="${MASTER_CONFIG_DIR}/openshift-router.kubeconfig" --config="${ADMIN_KUBECONFIG}" --images="${USE_IMAGES}" --service-account=router

# install the registry. The --mount-host option is provided to reuse local storage.
echo "[INFO] Installing the registry"
openshift admin registry --create --credentials="${MASTER_CONFIG_DIR}/openshift-registry.kubeconfig" --config="${ADMIN_KUBECONFIG}" --images="${USE_IMAGES}"

wait_for_command '[[ "$(oc get endpoints docker-registry --output-version=v1 -t "{{ if .subsets }}{{ len .subsets }}{{ else }}0{{ end }}" --config=/tmp/openshift-extended-tests/openshift.local.config/master/admin.kubeconfig || echo "0")" != "0" ]]' $((5*TIME_MIN))

echo "[INFO] Creating image streams"
oc create -n openshift -f examples/image-streams/image-streams-centos7.json --config="${ADMIN_KUBECONFIG}"

registry="$(dig @${API_HOST} "docker-registry.default.svc.cluster.local." +short A | head -n 1)"
echo "[INFO] Registry IP - ${registry}"

echo "[INFO] Starting extended tests ..."

# time go test ./test/extended/ #"${OS_ROOT}/hack/listtests.go" -prefix="${OS_GO_PACKAGE}/${package}.Test" "${testdir}"  | grep --color=never -E "${1-Test}" | xargs -I {} -n 1 bash -c "exectest {} ${@:2}" # "${testexec}" -test.run="^{}$" "${@:2}"
echo "[INFO] MASTER IP - ${MASTER_ADDR}"
echo "[INFO] SERVER CONFIG PATH - ${SERVER_CONFIG_DIR}"

KUBECONFIG="${ADMIN_KUBECONFIG}" \
	GOPATH="${OS_ROOT}/Godeps/_workspace:${GOPATH}" \
	go test -v -tags=default ./test/extended
