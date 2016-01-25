#!/bin/bash

# Provides simple utility functions
	
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

# time_now return the time since the epoch in millis
function time_now()
{
	echo $(date +%s000)
}

######
# start of common functions for extended test group's run.sh scripts
######

# exit run if ginkgo not installed
function ensure_ginkgo_or_die {
	which ginkgo &>/dev/null || (echo 'Run: "go get github.com/onsi/ginkgo/ginkgo"' && exit 1)
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
  	sed -i '' $@
  else
  	sed -i'' $@
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
