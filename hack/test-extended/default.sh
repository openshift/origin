#!/bin/bash
#
# This scripts starts the OpenShift server with a default configuration.
# The OpenShift Docker registry and router are installed.
# It will start all 'default_*_test.go' test cases.

set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..

source ${OS_ROOT}/hack/util.sh
source ${OS_ROOT}/hack/common.sh

echo "[INFO] Starting extended tests"

TIME_SEC=1000
TIME_MIN=$((60 * $TIME_SEC))

TMPDIR="${TMPDIR:-"/tmp"}"
BASETMPDIR="${TMPDIR}/openshift-extended-tests"

# Use either the latest release built images, or latest.
if [[ -z "${USE_IMAGES-}" ]]; then
	USE_IMAGES='openshift/origin-${component}:latest'
	if [[ -e "${OS_ROOT}/_output/local/releases/.commit" ]]; then
		COMMIT="$(cat "${OS_ROOT}/_output/local/releases/.commit")"
		USE_IMAGES="openshift/origin-\${component}:${COMMIT}"
	fi
fi


if [[ -z "${BASETMPDIR-}" ]]; then
	remove_tmp_dir && mkdir -p "${BASETMPDIR}"
fi

OS_TEST_NAMESPACE="extended-tests"

ETCD_DATA_DIR="${BASETMPDIR}/etcd"
VOLUME_DIR="${BASETMPDIR}/volumes"
FAKE_HOME_DIR="${BASETMPDIR}/openshift.local.home"
LOG_DIR="${LOG_DIR:-${BASETMPDIR}/logs}"
ARTIFACT_DIR="${ARTIFACT_DIR:-${BASETMPDIR}/artifacts}"
mkdir -p $LOG_DIR
mkdir -p $ARTIFACT_DIR

DEFAULT_SERVER_IP=`ifconfig | grep -Ev "(127.0.0.1|172.17.42.1)" | grep "inet " | head -n 1 | sed 's/adr://' | awk '{print $2}'`
API_HOST="${API_HOST:-${DEFAULT_SERVER_IP}}"
API_PORT="${API_PORT:-8443}"
API_SCHEME="${API_SCHEME:-https}"
MASTER_ADDR="${API_SCHEME}://${API_HOST}:${API_PORT}"
PUBLIC_MASTER_HOST="${PUBLIC_MASTER_HOST:-${API_HOST}}"
KUBELET_SCHEME="${KUBELET_SCHEME:-https}"
KUBELET_HOST="${KUBELET_HOST:-127.0.0.1}"
KUBELET_PORT="${KUBELET_PORT:-10250}"

SERVER_CONFIG_DIR="${BASETMPDIR}/openshift.local.config"
MASTER_CONFIG_DIR="${SERVER_CONFIG_DIR}/master"
NODE_CONFIG_DIR="${SERVER_CONFIG_DIR}/node-${KUBELET_HOST}"

# use the docker bridge ip address until there is a good way to get the auto-selected address from master
# this address is considered stable
# used as a resolve IP to test routing
CONTAINER_ACCESSIBLE_API_HOST="${CONTAINER_ACCESSIBLE_API_HOST:-172.17.42.1}"

STI_CONFIG_FILE="${LOG_DIR}/stiAppConfig.json"
DOCKER_CONFIG_FILE="${LOG_DIR}/dockerAppConfig.json"
CUSTOM_CONFIG_FILE="${LOG_DIR}/customAppConfig.json"
GO_OUT="${OS_ROOT}/_output/local/go/bin"

# set path so OpenShift is available
export PATH="${GO_OUT}:${PATH}"

cleanup() {
    set +e
    pid=$(cat ${BASETMPDIR}/server.pid 2>/dev/null)
    if [ ! -z "$pid" ]; then
      server_pids=$(pgrep -P $pid)
      kill $server_pids $(cat ${BASETMPDIR}/server.pid) ${ETCD_PID}
    fi
    rm -rf ${ETCD_DIR-}

	echo "[INFO] Stopping k8s docker containers"; docker ps | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker stop
	if [[ -z "${SKIP_IMAGE_CLEANUP-}" ]]; then
		echo "[INFO] Removing k8s docker containers"; docker ps -a | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker rm
	fi

	echo "[INFO] Removing ${BASETMPDIR}"
	rm -rf ${BASETMPDIR}
	if [[ $? != 0 ]]; then
		echo "[INFO] Unmounting volumes ..."
		findmnt -lo TARGET | grep openshift-${OS_TEST_NAMESPACE} | xargs -r sudo umount
		rm -rf ${BASETMPDIR}
	fi

	remove_tmp_dir
    echo "[INFO] Cleanup complete"
}

remove_tmp_dir() {
	rm -rf ${BASETMPDIR} &>/dev/null
	if [[ $? != 0 ]]; then
		echo "[INFO] Unmounting volumes ..."
		findmnt -lo TARGET | grep openshift-extended-tests | xargs -r sudo umount
		rm -rf ${BASETMPDIR}
	fi
}

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

openshift admin ca create-master-certs \
	--overwrite=false \
	--cert-dir="${MASTER_CONFIG_DIR}" \
	--hostnames="${SERVER_HOSTNAME_LIST}" \
	--master="${MASTER_ADDR}" \
	--public-master="${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}"

echo "[INFO] Creating OpenShift node config"
openshift admin create-node-config \
	--listen="${KUBELET_SCHEME}://0.0.0.0:${KUBELET_PORT}" \
	--node-dir="${NODE_CONFIG_DIR}" \
	--node="${KUBELET_HOST}" \
	--hostnames="${KUBELET_HOST}" \
	--master="${MASTER_ADDR}" \
	--node-client-certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" \
	--certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" \
	--signer-cert="${MASTER_CONFIG_DIR}/ca.crt" \
	--signer-key="${MASTER_CONFIG_DIR}/ca.key" \
	--signer-serial="${MASTER_CONFIG_DIR}/ca.serial.txt"

oadm create-bootstrap-policy-file --filename="${MASTER_CONFIG_DIR}/policy.json"

echo "[INFO] Creating OpenShift config"
openshift start \
	--write-config=${SERVER_CONFIG_DIR} \
	--create-certs=false \
    --listen="${API_SCHEME}://0.0.0.0:${API_PORT}" \
    --master="${MASTER_ADDR}" \
    --public-master="${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}" \
    --hostname="${KUBELET_HOST}" \
    --volume-dir="${VOLUME_DIR}" \
    --etcd-dir="${ETCD_DATA_DIR}" \
    --images="${USE_IMAGES}"


echo "[INFO] Starting OpenShift server"
sudo env "PATH=${PATH}" OPENSHIFT_PROFILE=web OPENSHIFT_ON_PANIC=crash openshift start \
	--master-config=${MASTER_CONFIG_DIR}/master-config.yaml \
	--node-config=${NODE_CONFIG_DIR}/node-config.yaml \
    --loglevel=4 \
    &> "${LOG_DIR}/openshift.log" &
OS_PID=$!

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


wait_for_url "${KUBELET_SCHEME}://${KUBELET_HOST}:${KUBELET_PORT}/healthz" "[INFO] kubelet: " 0.5 60
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1/nodes/${KUBELET_HOST}" "apiserver(nodes): " 0.25 80

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

MASTER_ADDR="${MASTER_ADDR}" \
	KUBECONFIG="${ADMIN_KUBECONFIG}" \
	GOPATH="${OS_ROOT}/Godeps/_workspace:/${GOPATH}" \
	go test -v -tags=default ./test/extended
