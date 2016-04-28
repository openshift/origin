#!/bin/bash

# Provides simple utility functions

# configure_os_server will create and write OS master certificates, node configurations, and OpenShift configurations.
# It is recommended to run the following environment setup functions before configuring the OpenShift server:
#  - os::util::environment::setup_all_server_vars
#  - os::util::environment::use_sudo -- if your script should be using root privileges
#
# Globals:
#  - ALL_IP_ADDRESSES
#  - PUBLIC_MASTER_HOST
#  - MASTER_CONFIG_DIR
#  - MASTER_ADDR
#  - API_SCHEME
#  - PUBLIC_MASTER_HOST
#  - API_PORT
#  - KUBELET_SCHEME
#  - KUBELET_BIND_HOST
#  - KUBELET_PORT
#  - NODE_CONFIG_DIR
#  - KUBELET_HOST
#  - API_BIND_HOST
#  - VOLUME_DIR
#  - ETCD_DATA_DIR
#  - USE_IMAGES
#  - USE_SUDO
# Arguments:
#  None
# Returns:
#  - export ADMIN_KUBECONFIG
#  - export CLUSTER_ADMIN_CONTEXT
function configure_os_server {
	# find the same IP that openshift start will bind to.	This allows access from pods that have to talk back to master
	if [[ -z "${ALL_IP_ADDRESSES-}" ]]; then
		ALL_IP_ADDRESSES="$(openshift start --print-ip)"
		SERVER_HOSTNAME_LIST="${PUBLIC_MASTER_HOST},localhost,172.30.0.1"
                SERVER_HOSTNAME_LIST="${SERVER_HOSTNAME_LIST},kubernetes.default.svc.cluster.local,kubernetes.default.svc,kubernetes.default,kubernetes"
                SERVER_HOSTNAME_LIST="${SERVER_HOSTNAME_LIST},openshift.default.svc.cluster.local,openshift.default.svc,openshift.default,openshift"

		while read -r IP_ADDRESS
		do
			SERVER_HOSTNAME_LIST="${SERVER_HOSTNAME_LIST},${IP_ADDRESS}"
		done <<< "${ALL_IP_ADDRESSES}"

		export ALL_IP_ADDRESSES
		export SERVER_HOSTNAME_LIST
	fi

	echo "[INFO] Creating certificates for the OpenShift server"
	openshift admin ca create-master-certs \
	--overwrite=false \
	--cert-dir="${MASTER_CONFIG_DIR}" \
	--hostnames="${SERVER_HOSTNAME_LIST}" \
	--master="${MASTER_ADDR}" \
	--public-master="${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}"

	echo "[INFO] Creating OpenShift node config"
	openshift admin create-node-config \
	--listen="${KUBELET_SCHEME}://${KUBELET_BIND_HOST}:${KUBELET_PORT}" \
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
	--listen="${API_SCHEME}://${API_BIND_HOST}:${API_PORT}" \
	--master="${MASTER_ADDR}" \
	--public-master="${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}" \
	--hostname="${KUBELET_HOST}" \
	--volume-dir="${VOLUME_DIR}" \
	--etcd-dir="${ETCD_DATA_DIR}" \
	--images="${USE_IMAGES}"


	# Don't try this at home.  We don't have flags for setting etcd ports in the config, but we want deconflicted ones.  Use sed to replace defaults in a completely unsafe way
	os::util::sed "s/:4001$/:${ETCD_PORT}/g" ${SERVER_CONFIG_DIR}/master/master-config.yaml
	os::util::sed "s/:7001$/:${ETCD_PEER_PORT}/g" ${SERVER_CONFIG_DIR}/master/master-config.yaml


	# Make oc use ${MASTER_CONFIG_DIR}/admin.kubeconfig, and ignore anything in the running user's $HOME dir
	export ADMIN_KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
	export CLUSTER_ADMIN_CONTEXT=$(oc config view --config=${ADMIN_KUBECONFIG} --flatten -o template --template='{{index . "current-context"}}')
	local sudo="${USE_SUDO:+sudo}"
	${sudo} chmod -R a+rwX "${ADMIN_KUBECONFIG}"
	echo "[INFO] To debug: export KUBECONFIG=$ADMIN_KUBECONFIG"
}


# start_os_server starts the OpenShift server, exports the PID of the OpenShift server and waits until openshift server endpoints are available
# It is advised to use this function after a successful run of 'configure_os_server'
#
# Globals:
#  - USE_SUDO
#  - LOG_DIR
#  - ARTIFACT_DIR
#  - VOLUME_DIR
#  - SERVER_CONFIG_DIR
#  - USE_IMAGES
#  - MASTER_ADDR
#  - MASTER_CONFIG_DIR
#  - NODE_CONFIG_DIR
#  - API_SCHEME
#  - API_HOST
#  - API_PORT
#  - KUBELET_SCHEME
#  - KUBELET_HOST
#  - KUBELET_PORT
# Arguments:
#  None
# Returns:
#  - export OS_PID
function start_os_server {
	local sudo="${USE_SUDO:+sudo}"

	local use_latest_images
	if [[ -n "${USE_LATEST_IMAGES:-}" ]]; then
		use_latest_images="true"
	else
		use_latest_images="false"
	fi

	echo "[INFO] `openshift version`"
	echo "[INFO] Server logs will be at:    ${LOG_DIR}/openshift.log"
	echo "[INFO] Test artifacts will be in: ${ARTIFACT_DIR}"
	echo "[INFO] Volumes dir is:            ${VOLUME_DIR}"
	echo "[INFO] Config dir is:             ${SERVER_CONFIG_DIR}"
	echo "[INFO] Using images:              ${USE_IMAGES}"
	echo "[INFO] MasterIP is:               ${MASTER_ADDR}"

	mkdir -p ${LOG_DIR}

	echo "[INFO] Scan of OpenShift related processes already up via ps -ef	| grep openshift : "
	ps -ef | grep openshift
	echo "[INFO] Starting OpenShift server"
	${sudo} env "PATH=${PATH}" OPENSHIFT_PROFILE=web OPENSHIFT_ON_PANIC=crash openshift start \
	 --dns="tcp://${API_HOST}:53" \
	 --master-config=${MASTER_CONFIG_DIR}/master-config.yaml \
	 --node-config=${NODE_CONFIG_DIR}/node-config.yaml \
	 --loglevel=4 --logspec='*importer=5' \
	 --latest-images="${use_latest_images}" \
	&>"${LOG_DIR}/openshift.log" &
	export OS_PID=$!

	echo "[INFO] OpenShift server start at: "
	date

	wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
	wait_for_url "${KUBELET_SCHEME}://${KUBELET_HOST}:${KUBELET_PORT}/healthz" "[INFO] kubelet: " 0.5 120
	wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 80
	wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1/nodes/${KUBELET_HOST}" "apiserver(nodes): " 0.25 80

	echo "[INFO] OpenShift server health checks done at: "
	date
}

# start_os_master starts the OpenShift master, exports the PID of the OpenShift master and waits until OpenShift master endpoints are available
# It is advised to use this function after a successful run of 'configure_os_server'
#
# Globals:
#  - USE_SUDO
#  - LOG_DIR
#  - ARTIFACT_DIR
#  - SERVER_CONFIG_DIR
#  - USE_IMAGES
#  - MASTER_ADDR
#  - MASTER_CONFIG_DIR
#  - API_SCHEME
#  - API_HOST
#  - API_PORT
# Arguments:
#  None
# Returns:
#  - export OS_PID
function start_os_master {
	local sudo="${USE_SUDO:+sudo}"

	echo "[INFO] `openshift version`"
	echo "[INFO] Server logs will be at:    ${LOG_DIR}/openshift.log"
	echo "[INFO] Test artifacts will be in: ${ARTIFACT_DIR}"
	echo "[INFO] Config dir is:             ${SERVER_CONFIG_DIR}"
	echo "[INFO] Using images:              ${USE_IMAGES}"
	echo "[INFO] MasterIP is:               ${MASTER_ADDR}"

	mkdir -p ${LOG_DIR}

	echo "[INFO] Scan of OpenShift related processes already up via ps -ef	| grep openshift : "
	ps -ef | grep openshift
	echo "[INFO] Starting OpenShift server"
	${sudo} env "PATH=${PATH}" OPENSHIFT_PROFILE=web OPENSHIFT_ON_PANIC=crash openshift start master \
	 --dns="tcp://${API_HOST}:53" \
	 --config=${MASTER_CONFIG_DIR}/master-config.yaml \
	 --loglevel=4 --logspec='*importer=5' \
	&>"${LOG_DIR}/openshift.log" &
	export OS_PID=$!

	echo "[INFO] OpenShift server start at: "
	date

	wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 160
	wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 160

	echo "[INFO] OpenShift server health checks done at: "
	date
}
# ensure_iptables_or_die tests if the testing machine has iptables available
# and in PATH. Also test whether current user has sudo privileges.
function ensure_iptables_or_die {
	if [[ -z "$(which iptables)" ]]; then
		echo "IPTables not found - the end-to-end test requires a system with iptables for Kubernetes services."
		exit 1
	fi

	set +e

	iptables --list > /dev/null 2>&1
	if [ $? -ne 0 ]; then
		sudo iptables --list > /dev/null 2>&1
		if [ $? -ne 0 ]; then
			echo "You do not have iptables or sudo privileges. Kubernetes services will not work without iptables access.	See https://github.com/kubernetes/kubernetes/issues/1859.	Try 'sudo hack/test-end-to-end.sh'."
			exit 1
		fi
	fi

	set -e
}

# tryuntil loops, retrying an action until it succeeds or times out after 90 seconds.
function tryuntil {
	timeout=$(($(date +%s) + 90))
	echo "++ Retrying until success or timeout: ${@}"
	while [ 1 ]; do
		if eval "${@}" >/dev/null 2>&1; then
			return 0
		fi
		if [[ $(date +%s) -gt $timeout ]]; then
			# run it one more time so we can display the output
			# for debugging, since above we /dev/null the output
			if eval "${@}"; then
				return 0
			fi
			echo "++ timed out"
			return 1
		fi
	done
}

# wait_for_command executes a command and waits for it to
# complete or times out after max_wait.
#
# $1 - The command to execute (e.g. curl -fs http://redhat.com)
# $2 - Optional maximum time to wait in ms before giving up (Default: 10000ms)
# $3 - Optional alternate command to determine if the wait should
#		exit before the max_wait
function wait_for_command {
	STARTTIME=$(date +%s)
	cmd=$1
	msg="Waiting for command to finish: '${cmd}'..."
	max_wait=${2:-10*TIME_SEC}
	fail=${3:-""}
	wait=0.2

	echo "[INFO] $msg"
	expire=$(($(time_now) + $max_wait))
	set +e
	while [[ $(time_now) -lt $expire ]]; do
	eval $cmd
	if [ $? -eq 0 ]; then
		set -e
		ENDTIME=$(date +%s)
		echo "[INFO] Success running command: '$cmd' after $(($ENDTIME - $STARTTIME)) seconds"
		return 0
	fi
	#check a failure condition where the success
	#command may never be evaulated before timing
	#out
	if [[ ! -z $fail ]]; then
		eval $fail
		if [ $? -eq 0 ]; then
		set -e
		echo "[FAIL] Returning early. Command Failed '$cmd'"
		return 1
		fi
	fi
	sleep $wait
	done
	echo "[ ERR] Gave up waiting for: '$cmd'"
	set -e
	return 1
}

# wait_for_url_timed attempts to access a url in order to
# determine if it is available to service requests.
#
# $1 - The URL to check
# $2 - Optional prefix to use when echoing a successful result
# $3 - Optional maximum time to wait before giving up (Default: 10s)
function wait_for_url_timed {
	STARTTIME=$(date +%s)
	url=$1
	prefix=${2:-}
	max_wait=${3:-10*TIME_SEC}
	wait=0.2
	expire=$(($(time_now) + $max_wait))
	set +e
	while [[ $(time_now) -lt $expire ]]; do
	out=$(curl --max-time 2 -fs $url 2>/dev/null)
	if [ $? -eq 0 ]; then
		set -e
		echo ${prefix}${out}
		ENDTIME=$(date +%s)
		echo "[INFO] Success accessing '$url' after $(($ENDTIME - $STARTTIME)) seconds"
		return 0
	fi
	sleep $wait
	done
	echo "ERROR: gave up waiting for $url"
	set -e
	return 1
}

# wait_for_file returns 0 if a file exists, 1 if it does not exist
#
# $1 - The file to check for existence
# $2 - Optional time to sleep between attempts (Default: 0.2s)
# $3 - Optional number of attemps to make (Default: 10)
function wait_for_file {
	file=$1
	wait=${2:-0.2}
	times=${3:-10}
	for i in $(seq 1 $times); do
	if [ -f "${file}" ]; then
		return 0
	fi
	sleep $wait
	done
	echo "ERROR: gave up waiting for file ${file}"
	return 1
}

# wait_for_url attempts to access a url in order to
# determine if it is available to service requests.
#
# $1 - The URL to check
# $2 - Optional prefix to use when echoing a successful result
# $3 - Optional time to sleep between attempts (Default: 0.2s)
# $4 - Optional number of attemps to make (Default: 10)
function wait_for_url {
	url=$1
	prefix=${2:-}
	wait=${3:-0.2}
	times=${4:-10}

	set_curl_args $wait $times

	set +e
	cmd="env -i CURL_CA_BUNDLE=${CURL_CA_BUNDLE:-} $(which curl) ${clientcert_args} -fs ${url}"
	for i in $(seq 1 $times); do
		out=$(${cmd})
		if [ $? -eq 0 ]; then
			set -e
			echo "${prefix}${out}"
			return 0
		fi
		sleep $wait
	done
	echo "ERROR: gave up waiting for ${url}"
	echo $(${cmd})
	set -e
	return 1
}

# set_curl_args tries to export CURL_ARGS for a program to use.
# will do a wait for the files to exist when using curl with
# SecureTransport (because we must convert the keys to a different
# form).
#
# $1 - Optional time to sleep between attempts (Default: 0.2s)
# $2 - Optional number of attemps to make (Default: 10)
function set_curl_args {
	wait=${1:-0.2}
	times=${2:-10}

	CURL_CERT=${CURL_CERT:-}
	CURL_KEY=${CURL_KEY:-}
	clientcert_args="${CURL_EXTRA:-} "

	if [ -n "${CURL_CERT}" ]; then
	 if [ -n "${CURL_KEY}" ]; then
	 if [[ `curl -V` == *"SecureTransport"* ]]; then
		 # Convert to a p12 cert for SecureTransport
		 export CURL_CERT_DIR=$(dirname "${CURL_CERT}")
		 export CURL_CERT_P12=${CURL_CERT_P12:-${CURL_CERT_DIR}/cert.p12}
		 export CURL_CERT_P12_PASSWORD=${CURL_CERT_P12_PASSWORD:-password}
		 if [ ! -f "${CURL_CERT_P12}" ]; then
		 wait_for_file "${CURL_CERT}" $wait $times
		 wait_for_file "${CURL_KEY}" $wait $times
		 openssl pkcs12 -export -inkey "${CURL_KEY}" -in "${CURL_CERT}" -out "${CURL_CERT_P12}" -password "pass:${CURL_CERT_P12_PASSWORD}"
		 fi
		 clientcert_args="--cert ${CURL_CERT_P12}:${CURL_CERT_P12_PASSWORD} ${CURL_EXTRA:-}"
	 else
		 clientcert_args="--cert ${CURL_CERT} --key ${CURL_KEY} ${CURL_EXTRA:-}"
	 fi
	 fi
	fi
	export CURL_ARGS="${clientcert_args}"
}

# Search for a regular expression in a HTTP response.
#
# $1 - a valid URL (e.g.: http://127.0.0.1:8080)
# $2 - a regular expression or text
function validate_response {
	url=$1
	expected_response=$2
	wait=${3:-0.2}
	times=${4:-10}

	set +e
	for i in $(seq 1 $times); do
	response=`curl $url`
	echo $response | grep -q "$expected_response"
	if [ $? -eq 0 ]; then
		echo "[INFO] Response is valid."
		set -e
		return 0
	fi
	sleep $wait
	done

	echo "[INFO] Response is invalid: $response"
	set -e
	return 1
}


# reset_tmp_dir will try to delete the testing directory.
# If it fails will unmount all the mounts associated with
# the test.
#
# $1 expression for which the mounts should be checked
reset_tmp_dir() {
	local sudo="${USE_SUDO:+sudo}"

	set +e
	${sudo} rm -rf ${BASETMPDIR} &>/dev/null
	if [[ $? != 0 ]]; then
		echo "[INFO] Unmounting previously used volumes ..."
		findmnt -lo TARGET | grep ${BASETMPDIR} | xargs -r ${sudo} umount
		${sudo} rm -rf ${BASETMPDIR}
	fi

	mkdir -p ${BASETMPDIR} ${LOG_DIR} ${ARTIFACT_DIR} ${FAKE_HOME_DIR} ${VOLUME_DIR}
	set -e
}

# kill_all_processes function will kill all
# all processes created by the test script.
function kill_all_processes()
{
	local sudo="${USE_SUDO:+sudo}"

	pids=($(jobs -pr))
	for i in ${pids[@]-}; do
		pgrep -P "${i}" | xargs $sudo kill &> /dev/null
		$sudo kill ${i} &> /dev/null
	done
}

# time_now return the time since the epoch in millis
function time_now()
{
	echo $(date +%s000)
}

# dump_container_logs writes container logs to $LOG_DIR
function dump_container_logs()
{
	if ! docker version >/dev/null 2>&1; then
		return
	fi

	mkdir -p ${LOG_DIR}

	echo "[INFO] Dumping container logs to ${LOG_DIR}"
	for container in $(docker ps -aq); do
		container_name=$(docker inspect -f "{{.Name}}" $container)
		# strip off leading /
		container_name=${container_name:1}
		if [[ "$container_name" =~ ^k8s_ ]]; then
			pod_name=$(echo $container_name | awk 'BEGIN { FS="[_.]+" }; { print $4 }')
			container_name=${pod_name}-$(echo $container_name | awk 'BEGIN { FS="[_.]+" }; { print $2 }')
		fi
		docker logs "$container" >&"${LOG_DIR}/container-${container_name}.log"
	done
}

# delete_empty_logs deletes empty logs
function delete_empty_logs() {
	# Clean up zero byte log files
	find "${ARTIFACT_DIR}" "${LOG_DIR}" -type f -name '*.log' \( -empty \) -delete
}

# truncate_large_logs truncates large logs so we only download the last 20MB
function truncate_large_logs() {
	# Clean up large log files so they don't end up on jenkins
	local large_files=$(find "${ARTIFACT_DIR}" "${LOG_DIR}" -type f -name '*.log' \( -size +20M \))
	for file in ${large_files}; do
		cp "${file}" "${file}.tmp"
		echo "LOGFILE TOO LONG, PREVIOUS BYTES TRUNCATED. LAST 20M BYTES OF LOGFILE:" > "${file}"
		tail -c 20M "${file}.tmp" >> "${file}"
		rm "${file}.tmp"
	done
}

######
# start of common functions for extended test group's run.sh scripts
######

# exit run if ginkgo not installed
function ensure_ginkgo_or_die {
	which ginkgo &>/dev/null || (echo 'Run: "go get github.com/onsi/ginkgo/ginkgo"' && exit 1)
}

# cleanup_openshift saves container logs, saves resources, and kills all processes and containers
function cleanup_openshift {
	ADMIN_KUBECONFIG="${ADMIN_KUBECONFIG:-${BASETMPDIR}/openshift.local.config/master/admin.kubeconfig}"
	LOG_DIR="${LOG_DIR:-${BASETMPDIR}/logs}"
	ARTIFACT_DIR="${ARTIFACT_DIR:-${LOG_DIR}}"
	API_HOST="${API_HOST:-127.0.0.1}"
	API_SCHEME="${API_SCHEME:-https}"
	ETCD_PORT="${ETCD_PORT:-4001}"

	set +e
	dump_container_logs

	if [[ -e "${ADMIN_KUBECONFIG:-}" ]]; then
		echo "[INFO] Dumping all resources to ${LOG_DIR}/export_all.json"
		oc login -u system:admin -n default --config=${ADMIN_KUBECONFIG}
		oc export all --all-namespaces --raw -o json --config=${ADMIN_KUBECONFIG} > ${LOG_DIR}/export_all.json
	fi

	echo "[INFO] Dumping etcd contents to ${ARTIFACT_DIR}/etcd_dump.json"
	set_curl_args 0 1
	curl -s ${clientcert_args} -L "${API_SCHEME}://${API_HOST}:${ETCD_PORT}/v2/keys/?recursive=true" > "${ARTIFACT_DIR}/etcd_dump.json"
	echo

	if [[ -z "${SKIP_TEARDOWN-}" ]]; then
		echo "[INFO] Tearing down test"
		kill_all_processes

		if docker version >/dev/null 2>&1; then
			echo "[INFO] Stopping k8s docker containers"; docker ps | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker stop -t 1 >/dev/null
			if [[ -z "${SKIP_IMAGE_CLEANUP-}" ]]; then
				echo "[INFO] Removing k8s docker containers"; docker ps -a | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker rm -v >/dev/null
			fi
		fi

		echo "[INFO] Pruning etcd data directory..."
		sudo rm -rf "${ETCD_DATA_DIR}"

		set -u
	fi

	# TODO soltysh: restore the if back once #8399 is resolved
	# if grep -q 'no Docker socket found' "${LOG_DIR}/openshift.log"; then
		# the Docker daemon crashed, we need the logs
	# journalctl --unit docker.service --since -4hours > "${LOG_DIR}/docker.log"
	# fi
	journalctl --unit docker.service --since -15minutes > "${LOG_DIR}/docker.log"

	delete_empty_logs
	truncate_large_logs

	echo "[INFO] Cleanup complete"
	set -e
}

# create a .gitconfig for test-cmd secrets
function create_gitconfig {
	USERNAME=sample-user
	PASSWORD=password
	BASETMPDIR="${BASETMPDIR:-"/tmp"}"
	GITCONFIG_DIR=$(mktemp -d ${BASETMPDIR}/test-gitconfig.XXXX)
	touch ${GITCONFIG_DIR}/.gitconfig
	git config --file ${GITCONFIG_DIR}/.gitconfig user.name ${USERNAME}
	git config --file ${GITCONFIG_DIR}/.gitconfig user.token ${PASSWORD}
	echo ${GITCONFIG_DIR}/.gitconfig
}

function create_valid_file {
	BASETMPDIR="${BASETMPDIR:-"/tmp"}"
	FILE_DIR=$(mktemp -d ${BASETMPDIR}/test-file.XXXX)
	touch ${FILE_DIR}/${1}
	echo ${FILE_DIR}/${1}
}

# install the router for the extended tests
function install_router {
	echo "[INFO] Installing the router"
	oadm policy add-scc-to-user privileged -z router --config="${ADMIN_KUBECONFIG}"
        # Create a TLS certificate for the router
        if [[ -n "${CREATE_ROUTER_CERT:-}" ]]; then
            echo "[INFO] Generating router TLS certificate"
            oadm ca create-server-cert --signer-cert=${MASTER_CONFIG_DIR}/ca.crt \
                 --signer-key=${MASTER_CONFIG_DIR}/ca.key \
                 --signer-serial=${MASTER_CONFIG_DIR}/ca.serial.txt \
                 --hostnames="*.${API_HOST}.xip.io" \
                 --cert=${MASTER_CONFIG_DIR}/router.crt --key=${MASTER_CONFIG_DIR}/router.key
            cat ${MASTER_CONFIG_DIR}/router.crt ${MASTER_CONFIG_DIR}/router.key \
                ${MASTER_CONFIG_DIR}/ca.crt > ${MASTER_CONFIG_DIR}/router.pem
            ROUTER_DEFAULT_CERT="--default-cert=${MASTER_CONFIG_DIR}/router.pem"
        fi
        openshift admin router --config="${ADMIN_KUBECONFIG}" --images="${USE_IMAGES}" --service-account=router ${ROUTER_DEFAULT_CERT-}

        # Set the SYN eater to make router reloads more robust
        if [[ -n "${DROP_SYN_DURING_RESTART:-}" ]]; then
            # Rewrite the DC for the router to add the environment variable into the pod definition
            echo "[INFO] Changing the router DC to drop SYN packets during a reload"
            oc set env dc/router -c router DROP_SYN_DURING_RESTART=true
        fi

}

# install registry for the extended tests
function install_registry {
	# The --mount-host option is provided to reuse local storage.
	echo "[INFO] Installing the registry"
	openshift admin registry --config="${ADMIN_KUBECONFIG}" --images="${USE_IMAGES}"
}

function wait_for_registry {
	wait_for_command '[[ "$(oc get endpoints docker-registry --output-version=v1 --template="{{ if .subsets }}{{ len .subsets }}{{ else }}0{{ end }}" --config=${ADMIN_KUBECONFIG} || echo "0")" != "0" ]]' $((5*TIME_MIN))
}


# Wait for builds to start
# $1 namespace
function os::build:wait_for_start() {
	echo "[INFO] Waiting for $1 namespace build to start"
	wait_for_command "oc get -n $1 builds | grep -i running" $((10*TIME_MIN)) "oc get -n $1 builds | grep -i -e failed -e error"
	BUILD_ID=`oc get -n $1 builds  --output-version=v1 --template="{{with index .items 0}}{{.metadata.name}}{{end}}"`
	echo "[INFO] Build ${BUILD_ID} started"
}

# Wait for builds to complete
# $1 namespace
function os::build:wait_for_end() {
	echo "[INFO] Waiting for $1 namespace build to complete"
	wait_for_command "oc get -n $1 builds | grep -i complete" $((10*TIME_MIN)) "oc get -n $1 builds | grep -i -e failed -e error"
	BUILD_ID=`oc get -n $1 builds --output-version=v1 --template="{{with index .items 0}}{{.metadata.name}}{{end}}"`
	echo "[INFO] Build ${BUILD_ID} finished"
	# TODO: fix
	set +e
	oc build-logs -n $1 $BUILD_ID > $LOG_DIR/$1build.log
	set -e
}

# enable-selinux/disable-selinux use the shared control variable
# SELINUX_DISABLED to determine whether to re-enable selinux after it
# has been disabled.  The goal is to allow temporary disablement of
# selinux enforcement while avoiding enabling enforcement in an
# environment where it is not already enabled.
SELINUX_DISABLED=0

function enable-selinux {
  if [ "${SELINUX_DISABLED}" = "1" ]; then
    os::log::info "Re-enabling selinux enforcement"
    sudo setenforce 1
    SELINUX_DISABLED=0
  fi
}

function disable-selinux {
  if selinuxenabled && [ "$(getenforce)" = "Enforcing" ]; then
    os::log::info "Temporarily disabling selinux enforcement"
    sudo setenforce 0
    SELINUX_DISABLED=1
  fi
}

######
# end of common functions for extended test group's run.sh scripts
######

# Handler for when we exit automatically on an error.
# Borrowed from https://gist.github.com/ahendrix/7030300
os::log::errexit() {
	local err="${PIPESTATUS[@]}"

	# If the shell we are in doesn't have errexit set (common in subshells) then
	# don't dump stacks.
	set +o | grep -qe "-o errexit" || return

	set +o xtrace
	local code="${1:-1}"
	os::log::error_exit "'${BASH_COMMAND}' exited with status $err" "${1:-1}" 1
}

os::log::install_errexit() {
	# trap ERR to provide an error handler whenever a command exits nonzero this
	# is a more verbose version of set -o errexit
	trap 'os::log::errexit' ERR

	# setting errtrace allows our ERR trap handler to be propagated to functions,
	# expansions and subshells
	set -o errtrace
}

# Print out the stack trace
#
# Args:
#	 $1 The number of stack frames to skip when printing.
os::log::stack() {
	local stack_skip=${1:-0}
	stack_skip=$((stack_skip + 1))
	if [[ ${#FUNCNAME[@]} -gt $stack_skip ]]; then
	echo "Call stack:" >&2
	local i
	for ((i=1 ; i <= ${#FUNCNAME[@]} - $stack_skip ; i++))
	do
		local frame_no=$((i - 1 + stack_skip))
		local source_file=${BASH_SOURCE[$frame_no]}
		local source_lineno=${BASH_LINENO[$((frame_no - 1))]}
		local funcname=${FUNCNAME[$frame_no]}
		echo "	$i: ${source_file}:${source_lineno} ${funcname}(...)" >&2
	done
	fi
}

# Log an error and exit.
# Args:
#	 $1 Message to log with the error
#	 $2 The error code to return
#	 $3 The number of stack frames to skip when printing.
os::log::error_exit() {
	local message="${1:-}"
	local code="${2:-1}"
	local stack_skip="${3:-0}"
	stack_skip=$((stack_skip + 1))

	local source_file=${BASH_SOURCE[$stack_skip]}
	local source_line=${BASH_LINENO[$((stack_skip - 1))]}
	echo "!!! Error in ${source_file}:${source_line}" >&2
	[[ -z ${1-} ]] || {
	echo "	${1}" >&2
	}

	os::log::stack $stack_skip

	echo "Exiting with status ${code}" >&2
	exit "${code}"
}

os::log::with-severity() {
  local msg=$1
  local severity=$2

  echo "[$2] ${1}"
}

os::log::info() {
  os::log::with-severity "${1}" "INFO"
}

os::log::warn() {
  os::log::with-severity "${1}" "WARNING"
}

os::log::error() {
  os::log::with-severity "${1}" "ERROR"
}

find_files() {
	find . -not \( \
		\( \
		-wholename './_output' \
		-o -wholename './_tools' \
		-o -wholename './.*' \
		-o -wholename './pkg/assets/bindata.go' \
		-o -wholename './pkg/assets/*/bindata.go' \
		-o -wholename './pkg/bootstrap/bindata.go' \
		-o -wholename './openshift.local.*' \
		-o -wholename '*/Godeps/*' \
		-o -wholename './assets/bower_components/*' \
		\) -prune \
	\) -name '*.go' | sort -u
}

# Asks golang what it thinks the host platform is.  The go tool chain does some
# slightly different things when the target platform matches the host platform.
os::util::host_platform() {
  echo "$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
}

os::util::sed() {
  if [[ "$(go env GOHOSTOS)" == "darwin" ]]; then
  	sed -i '' "$@"
  else
  	sed -i'' "$@"
  fi
}

os::util::base64decode() {
  if [[ "$(go env GOHOSTOS)" == "darwin" ]]; then
  	base64 -D $@
  else
  	base64 -d $@
  fi
}

os::util::get_object_assert() {
  local object=$1
  local request=$2
  local expected=$3

  res=$(eval oc get $object -o go-template=\"$request\")

  if [[ "$res" =~ ^$expected$ ]]; then
      echo "Successful get $object $request: $res"
      return 0
  else
      echo "FAIL!"
      echo "Get $object $request"
      echo "  Expected: $expected"
      echo "  Got:      $res"
      return 1
  fi
}
