#!/bin/bash

# This script tests the high level end-to-end functionality demonstrated
# as part of the examples/sample-app

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/lib/log.sh"
source "${OS_ROOT}/hack/lib/util/environment.sh"

echo "[INFO] Starting containerized end-to-end test"

unset KUBECONFIG

os::util::environment::setup_all_server_vars "test-end-to-end-docker/"
os::util::environment::use_sudo
reset_tmp_dir

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

	echo "[INFO] Dumping all resources to ${LOG_DIR}/export_all.json"
	oc export all --all-namespaces --raw -o json --config=${ADMIN_KUBECONFIG} > ${LOG_DIR}/export_all.json

	echo "[INFO] Dumping etcd contents to ${ARTIFACT_DIR}/etcd_dump.json"
	set_curl_args 0 1
	ETCD_PORT="${ETCD_PORT:-4001}"
	curl ${clientcert_args} -L "${API_SCHEME}://${API_HOST}:${ETCD_PORT}/v2/keys/?recursive=true" > "${ARTIFACT_DIR}/etcd_dump.json"
	echo

	if [[ -z "${SKIP_TEARDOWN-}" ]]; then
		echo "[INFO] remove the openshift container"
		docker stop origin
		docker rm origin

		echo "[INFO] Stopping k8s docker containers"; docker ps | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker stop
		if [[ -z "${SKIP_IMAGE_CLEANUP-}" ]]; then
			echo "[INFO] Removing k8s docker containers"; docker ps -a | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker rm
		fi
		set -u
	fi

	# TODO soltysh: restore the if back once #8399 is resolved
	journalctl --unit docker.service --since -15minutes > "${LOG_DIR}/docker.log"

	delete_empty_logs
	truncate_large_logs
	set -e

	echo "[INFO] Exiting"
	ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"
	exit $out
}

trap "cleanup" EXIT INT TERM

os::log::start_system_logger

out=$(
	set +e
	docker stop origin 2>&1
	docker rm origin 2>&1
	set -e
)

# Setup
echo "[INFO] `openshift version`"
echo "[INFO] Using images:							${USE_IMAGES}"

echo "[INFO] Starting OpenShift containerized server"
sudo docker run -d --name="origin" \
	--privileged --net=host --pid=host \
	-v /:/rootfs:ro -v /var/run:/var/run:rw -v /sys:/sys:ro -v /var/lib/docker:/var/lib/docker:rw \
	-v "${VOLUME_DIR}:${VOLUME_DIR}" -v "${VOLUME_DIR}/etcd":/var/lib/origin/openshift.local.etcd:rw \
	"openshift/origin:${TAG}" start --dns="tcp://${API_HOST}":53 --loglevel=4 --volume-dir=${VOLUME_DIR} --images="${USE_IMAGES}"


# the CA is in the container, log in as a different cluster admin to run the test
CURL_EXTRA="-k"
wait_for_url "https://localhost:8443/healthz/ready" "apiserver(ready): " 0.25 160

IMAGE_WORKING_DIR=/var/lib/origin
docker cp origin:${IMAGE_WORKING_DIR}/openshift.local.config ${BASETMPDIR}

export ADMIN_KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
export CLUSTER_ADMIN_CONTEXT=$(oc config view --config=${ADMIN_KUBECONFIG} --flatten -o template --template='{{index . "current-context"}}')
sudo chmod -R a+rwX "${ADMIN_KUBECONFIG}"
export KUBECONFIG="${ADMIN_KUBECONFIG}"
echo "[INFO] To debug: export KUBECONFIG=$ADMIN_KUBECONFIG"


wait_for_url "${KUBELET_SCHEME}://${KUBELET_HOST}:${KUBELET_PORT}/healthz" "[INFO] kubelet: " 0.5 60
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 80


${OS_ROOT}/test/end-to-end/core.sh
