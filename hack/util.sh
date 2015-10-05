#!/bin/bash

# Provides simple utility functions

TIME_SEC=1000
TIME_MIN=$((60 * $TIME_SEC))

# setup_env_vars exports all the necessary environment variables for configuring and
# starting OS server.
function setup_env_vars {
	# set path so OpenShift is available
	GO_OUT="${OS_ROOT}/_output/local/bin/$(os::util::host_platform)"
	export PATH="${GO_OUT}:${PATH}"

	export ETCD_PORT="${ETCD_PORT:-4001}"
	export ETCD_PEER_PORT="${ETCD_PEER_PORT:-7001}"
	export API_HOST="${API_HOST:-$(openshift start --print-ip)}"
	export API_PORT="${API_PORT:-8443}"
	export LOG_DIR="${LOG_DIR:-${BASETMPDIR}/logs}"
	export ETCD_DATA_DIR="${BASETMPDIR}/etcd"
	export VOLUME_DIR="${BASETMPDIR}/volumes"
	export FAKE_HOME_DIR="${BASETMPDIR}/openshift.local.home"
	export API_SCHEME="${API_SCHEME:-https}"
	export MASTER_ADDR="${API_SCHEME}://${API_HOST}:${API_PORT}"
	export PUBLIC_MASTER_HOST="${PUBLIC_MASTER_HOST:-${API_HOST}}"
	export KUBELET_SCHEME="${KUBELET_SCHEME:-https}"
	export KUBELET_HOST="${KUBELET_HOST:-127.0.0.1}"
	export KUBELET_PORT="${KUBELET_PORT:-10250}"
	export SERVER_CONFIG_DIR="${BASETMPDIR}/openshift.local.config"
	export MASTER_CONFIG_DIR="${SERVER_CONFIG_DIR}/master"
	export NODE_CONFIG_DIR="${SERVER_CONFIG_DIR}/node-${KUBELET_HOST}"
	export ARTIFACT_DIR="${ARTIFACT_DIR:-${BASETMPDIR}/artifacts}"
	if [ -z ${SUDO+x} ]; then
		export SUDO="${SUDO:-1}"
	fi

	# Use either the latest release built images, or latest.
	if [[ -z "${USE_IMAGES-}" ]]; then
		IMAGES='openshift/origin-${component}:latest'
		export TAG=latest
		export USE_IMAGES=${IMAGES}
		if [[ -e "${OS_ROOT}/_output/local/releases/.commit" ]]; then
			COMMIT="$(cat "${OS_ROOT}/_output/local/releases/.commit")"
			IMAGES="openshift/origin-\${component}:${COMMIT}"
			export TAG=${COMMIT}
			export USE_IMAGES=${IMAGES}
		fi
	fi

	if [[ "${API_SCHEME}" == "https" ]]; then
		export CURL_CA_BUNDLE="${MASTER_CONFIG_DIR}/ca.crt"
		export CURL_CERT="${MASTER_CONFIG_DIR}/admin.crt"
		export CURL_KEY="${MASTER_CONFIG_DIR}/admin.key"
	fi

	# change the location of $HOME so no one does anything naughty
	export HOME="${FAKE_HOME_DIR}"
}

# configure_and_start_os will create and write OS master certificates, node config,
# OS config.
function configure_os_server {
	# find the same IP that openshift start will bind to.	This allows access from pods that have to talk back to master
	if [[ -z "${ALL_IP_ADDRESSES-}" ]]; then
		ALL_IP_ADDRESSES=`ifconfig | grep "inet " | sed 's/adr://' | awk '{print $2}'`
		SERVER_HOSTNAME_LIST="${PUBLIC_MASTER_HOST},localhost"

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


	# Don't try this at home.  We don't have flags for setting etcd ports in the config, but we want deconflicted ones.  Use sed to replace defaults in a completely unsafe way
	os::util::sed "s/:4001$/:${ETCD_PORT}/g" ${SERVER_CONFIG_DIR}/master/master-config.yaml
	os::util::sed "s/:7001$/:${ETCD_PEER_PORT}/g" ${SERVER_CONFIG_DIR}/master/master-config.yaml


	# Make oc use ${MASTER_CONFIG_DIR}/admin.kubeconfig, and ignore anything in the running user's $HOME dir
	export ADMIN_KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
	export CLUSTER_ADMIN_CONTEXT=$(oc config view --config=${ADMIN_KUBECONFIG} --flatten -o template --template='{{index . "current-context"}}')
	local sudo="${SUDO:+sudo}"
	${sudo} chmod -R a+rwX "${ADMIN_KUBECONFIG}"
	echo "[INFO] To debug: export KUBECONFIG=$ADMIN_KUBECONFIG"
}


# start_os_server starts the OS server, exports the PID of the OS server
# and waits until OS server endpoints are available
function start_os_server {
	local sudo="${SUDO:+sudo}"

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
	 --master-config=${MASTER_CONFIG_DIR}/master-config.yaml \
	 --node-config=${NODE_CONFIG_DIR}/node-config.yaml \
	 --loglevel=4 \
	&> "${LOG_DIR}/openshift.log" &
	export OS_PID=$!

	echo "[INFO] OpenShift server start at: "
	date
	
	wait_for_url "${KUBELET_SCHEME}://${KUBELET_HOST}:${KUBELET_PORT}/healthz" "[INFO] kubelet: " 0.5 60
	wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
	wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 80
	wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1/nodes/${KUBELET_HOST}" "apiserver(nodes): " 0.25 80
	
	echo "[INFO] OpenShift server health checks done at: "
	date
}

# start_os_master starts the OS server, exports the PID of the OS server
# and waits until OS server endpoints are available
function start_os_master {
	local sudo="${SUDO:+sudo}"

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
	 --config=${MASTER_CONFIG_DIR}/master-config.yaml \
	 --loglevel=4 \
	&> "${LOG_DIR}/openshift.log" &
	export OS_PID=$!

	echo "[INFO] OpenShift server start at: "
	date
	
	wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
	wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 80
	
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

# tryuntil loops up to 60 seconds to redo this action
function tryuntil {
	timeout=$(($(date +%s) + 60))
	until eval "${@}" || [[ $(date +%s) -gt $timeout ]]; do :; done
}

# wait_for_command executes a command and waits for it to
# complete or times out after max_wait.
#
# $1 - The command to execute (e.g. curl -fs http://redhat.com)
# $2 - Optional maximum time to wait before giving up (Default: 10s)
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
	local sudo="${SUDO:+sudo}"

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
	local sudo="${SUDO:+sudo}"

	pids=($(jobs -pr))
	for i in ${pids[@]-}; do
		ps --ppid=${i} | xargs $sudo kill &> /dev/null
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

# delete_large_and_empty_logs deletes empty logs and logs over 20MB
function delete_large_and_empty_logs()
{
	# clean up zero byte log files
	# Clean up large log files so they don't end up on jenkins
	find ${ARTIFACT_DIR} -name *.log -size +20M -exec -exec rm -f {} \;
	find ${LOG_DIR} -name *.log -size +20M -exec -exec rm -f {} \;
	find ${LOG_DIR} -name *.log -size 0 -exec rm -f {} \;
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
	
	echo "[INFO] Dumping all resources to ${LOG_DIR}/export_all.json"
	oc export all --all-namespaces --raw -o json --config=${ADMIN_KUBECONFIG} > ${LOG_DIR}/export_all.json

	echo "[INFO] Dumping etcd contents to ${ARTIFACT_DIR}/etcd_dump.json"
	set_curl_args 0 1
	curl -s ${clientcert_args} -L "${API_SCHEME}://${API_HOST}:${ETCD_PORT}/v2/keys/?recursive=true" > "${ARTIFACT_DIR}/etcd_dump.json"
	echo

	if [[ -z "${SKIP_TEARDOWN-}" ]]; then
		echo "[INFO] Tearing down test"
		kill_all_processes

		echo "[INFO] Stopping k8s docker containers"; docker ps | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker stop -t 1 >/dev/null
		if [[ -z "${SKIP_IMAGE_CLEANUP-}" ]]; then
			echo "[INFO] Removing k8s docker containers"; docker ps -a | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker rm >/dev/null
		fi
		set -u
	fi

	delete_large_and_empty_logs

	echo "[INFO] Cleanup complete"
	set -e
}

# create a .gitconfig for test-cmd secrets
function create_gitconfig {
	USERNAME=sample-user
	PASSWORD=password
	GITCONFIG_DIR=$(mktemp -d /tmp/test-gitconfig.XXXX)
	touch ${GITCONFIG_DIR}/.gitconfig
	git config --file ${GITCONFIG_DIR}/.gitconfig user.name ${USERNAME}
	git config --file ${GITCONFIG_DIR}/.gitconfig user.token ${PASSWORD}
	echo ${GITCONFIG_DIR}/.gitconfig
}

function create_valid_file {
	FILE_DIR=$(mktemp -d /tmp/test-file.XXXX)
	touch ${FILE_DIR}/${1}
	echo ${FILE_DIR}/${1}
}

# install the router for the extended tests
function install_router {
	echo "[INFO] Installing the router"
	echo '{"kind":"ServiceAccount","apiVersion":"v1","metadata":{"name":"router"}}' | oc create -f - --config="${ADMIN_KUBECONFIG}"
	oc get scc privileged -o json --config="${ADMIN_KUBECONFIG}" | sed '/\"users\"/a \"system:serviceaccount:default:router\",' | oc replace scc privileged -f - --config="${ADMIN_KUBECONFIG}"
        # Create a TLS certificate for the router
        if [[ -n "${CREATE_ROUTER_CERT-}" ]]; then
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
        openshift admin router --create --credentials="${MASTER_CONFIG_DIR}/openshift-router.kubeconfig" --config="${ADMIN_KUBECONFIG}" --images="${USE_IMAGES}" --service-account=router ${ROUTER_DEFAULT_CERT-}
}

# install registry for the extended tests
function install_registry {
	# The --mount-host option is provided to reuse local storage.
	echo "[INFO] Installing the registry"
	openshift admin registry --create --credentials="${MASTER_CONFIG_DIR}/openshift-registry.kubeconfig" --config="${ADMIN_KUBECONFIG}" --images="${USE_IMAGES}"
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
	BUILD_ID=`oc get -n $1 builds --output-version=v1beta3 --template="{{with index .items 0}}{{.metadata.name}}{{end}}"`
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
    setenforce 1
    SELINUX_DISABLED=0
  fi
}

function disable-selinux {
  if selinuxenabled; then
    os::log::info "Temporarily disabling selinux enforcement"
    setenforce 0
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
	# trap ERR to provide an error handler whenever a command exits nonzero	this
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

find_files() {
	find . -not \( \
		\( \
		-wholename './_output' \
		-o -wholename './_tools' \
		-o -wholename './.*' \
		-o -wholename './pkg/assets/bindata.go' \
		-o -wholename './pkg/assets/*/bindata.go' \
		-o -wholename './openshift.local.*' \
		-o -wholename '*/Godeps/*' \
		\) -prune \
	\) -name '*.go' | sort -u
}

os::util::run-extended-tests() {
  local config_root=$1
  local focus_regex=$2
  local skip_regex=${3:-}
  local log_path=${4:-}

  export KUBECONFIG="${config_root}/openshift.local.config/master/admin.kubeconfig"
  export EXTENDED_TEST_PATH="${OS_ROOT}/test/extended"

  local test_cmd="ginkgo -progress -stream -v -focus=\"${focus_regex}\" \
-skip=\"${skip_regex}\" ${OS_OUTPUT_BINPATH}/extended.test"
  if [ "${log_path}" != "" ]; then
    test_cmd="${test_cmd} | tee ${log_path}"
  fi

  pushd "${EXTENDED_TEST_PATH}" > /dev/null
    eval "${test_cmd}; "'exit_status=${PIPESTATUS[0]}'
  popd > /dev/null

  return ${exit_status}
}

os::util::run-net-extended-tests() {
  local config_root=$1
  local focus_regex=${2:-.etworking[:]*}
  local skip_regex=${3:-}
  local log_path=${4:-}

  if [ -z "${skip_regex}" ]; then
      # The intra-pod test is currently broken for origin.
      skip_regex='Networking.*intra-pod'
      # Only the multitenant plugin can pass the isolation test
      if ! grep -q 'redhat/openshift-ovs-multitenant' \
           $(find "${config_root}" -name 'node-config.yaml' | head -n 1); then
        skip_regex="(${skip_regex}|networking: isolation)"
      fi
  fi

  os::util::run-extended-tests "${config_root}" "${focus_regex}" \
    "${skip_regex}" "${log_path}"
}

# Asks golang what it thinks the host platform is.  The go tool chain does some
# slightly different things when the target platform matches the host platform.
os::util::host_platform() {
  echo "$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
}

os::util::sed() {
  if [[ "$(go env GOHOSTOS)" == "darwin" ]]; then
  	sed -i '' $@
  else
  	sed -i'' $@
  fi
}
