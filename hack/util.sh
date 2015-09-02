#!/bin/bash

# Provides simple utility functions

TIME_SEC=1000
TIME_MIN=$((60 * $TIME_SEC))

# setup_env_vars exports all the necessary environment variables for configuring and
# starting OS server.
function setup_env_vars {
  export ETCD_DATA_DIR="${BASETMPDIR}/etcd"
  export VOLUME_DIR="${BASETMPDIR}/volumes"
  export FAKE_HOME_DIR="${BASETMPDIR}/openshift.local.home"
  export API_HOST="${API_HOST:-127.0.0.1}"
  export API_PORT="${API_PORT:-8443}"
  export API_SCHEME="${API_SCHEME:-https}"
  export MASTER_ADDR="${API_SCHEME}://${API_HOST}:${API_PORT}"
  export PUBLIC_MASTER_HOST="${PUBLIC_MASTER_HOST:-${API_HOST}}"
  export KUBELET_SCHEME="${KUBELET_SCHEME:-https}"
  export KUBELET_HOST="${KUBELET_HOST:-127.0.0.1}"
  export KUBELET_PORT="${KUBELET_PORT:-10250}"
  export SERVER_CONFIG_DIR="${BASETMPDIR}/openshift.local.config"
  export MASTER_CONFIG_DIR="${SERVER_CONFIG_DIR}/master"
  export NODE_CONFIG_DIR="${SERVER_CONFIG_DIR}/node-${KUBELET_HOST}"

  # set path so OpenShift is available
  GO_OUT="${OS_ROOT}/_output/local/go/bin"
  export PATH="${GO_OUT}:${PATH}"
}

# configure_and_start_os will create and write OS master certificates, node config,
# OS config.
function configure_os_server {
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
}


# start_os_server starts the OS server, exports the PID of the OS server
# and waits until OS server endpoints are available
function start_os_server {
    echo "[INFO] Scan of OpenShift related processes already up via ps -ef  | grep openshift : "
    ps -ef | grep openshift
    echo "[INFO] Starting OpenShift server"
    sudo env "PATH=${PATH}" OPENSHIFT_PROFILE=web OPENSHIFT_ON_PANIC=crash openshift start \
	 --master-config=${MASTER_CONFIG_DIR}/master-config.yaml \
	 --node-config=${NODE_CONFIG_DIR}/node-config.yaml \
	 --loglevel=4 \
    &> "${LOG_DIR}/openshift.log" &
    export OS_PID=$!

    echo "[INFO] OpenShift server start at: "
    echo `date`
  
    wait_for_url "${KUBELET_SCHEME}://${KUBELET_HOST}:${KUBELET_PORT}/healthz" "[INFO] kubelet: " 0.5 60
    wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
    wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 80
    wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1/nodes/${KUBELET_HOST}" "apiserver(nodes): " 0.25 80
    
    echo "[INFO] OpenShift server health checks done at: "
    echo `date`
}

# test_privileges tests if the testing machine has iptables available
# and in PATH. Also test whether current user has sudo privileges.  
function test_privileges {
  if [[ -z "$(which iptables)" ]]; then
    echo "IPTables not found - the end-to-end test requires a system with iptables for Kubernetes services."
    exit 1
  fi

  set +e

  iptables --list > /dev/null 2>&1
  if [ $? -ne 0 ]; then
    sudo iptables --list > /dev/null 2>&1
    if [ $? -ne 0 ]; then
      echo "You do not have iptables or sudo privileges. Kubernetes services will not work without iptables access.  See https://github.com/kubernetes/kubernetes/issues/1859.  Try 'sudo hack/test-end-to-end.sh'."
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
#      exit before the max_wait
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
  echo "wait_for_url - run: ${cmd}"
  for i in $(seq 1 $times); do
    out=$(${cmd})
    if [ $? -eq 0 ]; then
      set -e
      echo "${prefix}${out}"
      return 0
    fi
    echo " wait_for_url: ${cmd} non zero rc: ${out}"
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


# start_etcd starts an etcd server
# $1 - Optional host (Default: 127.0.0.1)
# $2 - Optional port (Default: 4001)
function start_etcd {
  [ ! -z "${ETCD_STARTED-}" ] && return

  host=${ETCD_HOST:-127.0.0.1}
  port=${ETCD_PORT:-4001}

  set +e

  if [ "$(which etcd 2>/dev/null)" == "" ]; then
    if [[ ! -f ${OS_ROOT}/_tools/etcd/bin/etcd ]]; then
      echo "etcd must be in your PATH or installed in _tools/etcd/bin/ with hack/install-etcd.sh"
      exit 1
    fi
    export PATH="${OS_ROOT}/_tools/etcd/bin:$PATH"
  fi

  running_etcd=$(ps -ef | grep etcd | grep -c name)
  if [ "$running_etcd" != "0" ]; then
    echo "etcd appears to already be running on this machine, please kill and restart the test."
    exit 1
  fi

  # Stop on any failures
  set -e

  # get etcd version
  etcd_version=$(etcd --version | awk '{print $3}')
  initial_cluster=""
  if [[ "${etcd_version}" =~ ^2 ]]; then
    initial_cluster="--initial-cluster test=http://localhost:2380,test=http://localhost:7001"
  fi

  # Start etcd
  export ETCD_DIR=$(mktemp -d -t test-etcd.XXXXXX)
  etcd -name test -data-dir ${ETCD_DIR} -bind-addr ${host}:${port} ${initial_cluster} >/dev/null 2>/dev/null &
  export ETCD_PID=$!

  wait_for_url "http://${host}:${port}/version" "etcd: " 0.25 80
  curl -X PUT  "http://${host}:${port}/v2/keys/_test"
  echo
}

# remove_tmp_dir will try to delete the testing directory.
# If it fails will unmount all the mounts associated with 
# the test.
# 
# $1 expression for which the mounts should be checked 
remove_tmp_dir() {
  set +e
  sudo rm -rf ${BASETMPDIR} &>/dev/null
  if [[ $? != 0 ]]; then
    echo "[INFO] Unmounting previously used volumes ..."
    findmnt -lo TARGET | grep $1 | xargs -r sudo umount
    sudo rm -rf ${BASETMPDIR}
  fi
  set -e
}

# kill_all_processes function will kill all 
# all processes created by the test script.
function kill_all_processes()
{
  sudo=
  if type sudo &> /dev/null; then
    sudo=sudo
  fi

  pids=($(jobs -pr))
  for i in ${pids[@]}; do
    ps --ppid=${i} | xargs $sudo kill &> /dev/null
    $sudo kill ${i} &> /dev/null &> /dev/null
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
  find ${ARTIFACT_DIR} -name *.log -size +20M -exec echo Deleting {} because it is too big. \; -exec rm -f {} \;
  find ${LOG_DIR} -name *.log -size +20M -exec echo Deleting {} because it is too big. \; -exec rm -f {} \;
  find ${LOG_DIR} -name *.log -size 0 -exec echo Deleting {} because it is empty. \; -exec rm -f {} \;
}

######
# start of common functions for extended test group's run.sh scripts
######

# exit run if ginkgo not installed
function ginkgo_check_extended {
    which ginkgo &>/dev/null || (echo 'Run: "go get github.com/onsi/ginkgo/ginkgo"' && exit 1)
}

# various env var and setup for directories and image related information
function dirs_image_env_setup_extended {
    export TIME_SEC=1000
    export TIME_MIN=$((60 * $TIME_SEC))

    export TEST_TYPE="openshift-extended-tests"
    export TMPDIR="${TMPDIR:-"/tmp"}"
    export BASETMPDIR="${TMPDIR}/${TEST_TYPE}"

    if [[ -d "${BASETMPDIR}" ]]; then
	remove_tmp_dir $TEST_TYPE
    fi

    mkdir -p ${BASETMPDIR}

    # Use either the latest release built images, or latest.
    if [[ -z "${USE_IMAGES-}" ]]; then
	export USE_IMAGES='openshift/origin-${component}:latest'
	if [[ -e "${OS_ROOT}/_output/local/releases/.commit" ]]; then
	    export COMMIT="$(cat "${OS_ROOT}/_output/local/releases/.commit")"
	    export USE_IMAGES="openshift/origin-\${component}:${COMMIT}"
	fi
    fi

    export LOG_DIR="${LOG_DIR:-${BASETMPDIR}/logs}"
    export ARTIFACT_DIR="${ARTIFACT_DIR:-${BASETMPDIR}/artifacts}"
    export DEFAULT_SERVER_IP=`ifconfig | grep -Ev "(127.0.0.1|172.17.42.1)" | grep "inet " | head -n 1 | sed 's/adr://' | awk '{print $2}'`
    export API_HOST="${API_HOST:-${DEFAULT_SERVER_IP}}"
    mkdir -p $LOG_DIR $ARTIFACT_DIR
}

# cleanup function for the extended tests
function cleanup_extended {
  out=$?
  set +e

  echo "[INFO] Tearing down test"
  kill_all_processes
  rm -rf ${ETCD_DIR-}
  echo "[INFO] Stopping k8s docker containers"; docker ps | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker stop

  dump_container_logs

  if [[ -z "${SKIP_IMAGE_CLEANUP-}" ]]; then
    echo "[INFO] Removing k8s docker containers"; docker ps -a | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker rm
  fi

  set -e
  echo "[INFO] Cleanup complete"
  echo "[INFO] Exiting"
  exit $out
}    

# ip/host setup function for the extended tests, along with some debug
function info_msgs_ip_host_setup_extended {
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
    export ALL_IP_ADDRESSES
    export SERVER_HOSTNAME_LIST
}

# auth set up for extended tests
function auth_setup_extended {
    export HOME="${FAKE_HOME_DIR}"
    # This directory must exist so Docker can store credentials in $HOME/.dockercfg
    mkdir -p ${FAKE_HOME_DIR}

    export ADMIN_KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
    export CLUSTER_ADMIN_CONTEXT=$(oc config view --flatten -o template -t '{{index . "current-context"}}')

    if [[ "${API_SCHEME}" == "https" ]]; then
	export CURL_CA_BUNDLE="${MASTER_CONFIG_DIR}/ca.crt"
	export CURL_CERT="${MASTER_CONFIG_DIR}/admin.crt"
	export CURL_KEY="${MASTER_CONFIG_DIR}/admin.key"

	# Make oc use ${MASTER_CONFIG_DIR}/admin.kubeconfig, and ignore anything in the running user's $HOME dir
	sudo chmod -R a+rwX "${ADMIN_KUBECONFIG}"
	echo "[INFO] To debug: export ADMIN_KUBECONFIG=$ADMIN_KUBECONFIG"
    fi

}

# install the router for the extended tests
function install_router_extended {
    echo "[INFO] Installing the router"
    echo '{"kind":"ServiceAccount","apiVersion":"v1","metadata":{"name":"router"}}' | oc create -f - --config="${ADMIN_KUBECONFIG}"
    oc get scc privileged -o json --config="${ADMIN_KUBECONFIG}" | sed '/\"users\"/a \"system:serviceaccount:default:router\",' | oc replace scc privileged -f - --config="${ADMIN_KUBECONFIG}"
    openshift admin router --create --credentials="${MASTER_CONFIG_DIR}/openshift-router.kubeconfig" --config="${ADMIN_KUBECONFIG}" --images="${USE_IMAGES}" --service-account=router
}

# install registry for the extended tests
function install_registry_extended {
    # The --mount-host option is provided to reuse local storage.
    echo "[INFO] Installing the registry"
    openshift admin registry --create --credentials="${MASTER_CONFIG_DIR}/openshift-registry.kubeconfig" --config="${ADMIN_KUBECONFIG}" --images="${USE_IMAGES}"
}

# create the images streams for the extended tests suites
function create_image_streams_extended {
    echo "[INFO] Creating image streams"
    oc create -n openshift -f examples/image-streams/image-streams-centos7.json --config="${ADMIN_KUBECONFIG}"

    registry="$(dig @${API_HOST} "docker-registry.default.svc.cluster.local." +short A | head -n 1)"
    echo "[INFO] Registry IP - ${registry}"
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
  # trap ERR to provide an error handler whenever a command exits nonzero  this
  # is a more verbose version of set -o errexit
  trap 'os::log::errexit' ERR

  # setting errtrace allows our ERR trap handler to be propagated to functions,
  # expansions and subshells
  set -o errtrace
}

# Print out the stack trace
#
# Args:
#   $1 The number of stack frames to skip when printing.
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
      echo "  $i: ${source_file}:${source_lineno} ${funcname}(...)" >&2
    done
  fi
}

# Log an error and exit.
# Args:
#   $1 Message to log with the error
#   $2 The error code to return
#   $3 The number of stack frames to skip when printing.
os::log::error_exit() {
  local message="${1:-}"
  local code="${2:-1}"
  local stack_skip="${3:-0}"
  stack_skip=$((stack_skip + 1))

  local source_file=${BASH_SOURCE[$stack_skip]}
  local source_line=${BASH_LINENO[$((stack_skip - 1))]}
  echo "!!! Error in ${source_file}:${source_line}" >&2
  [[ -z ${1-} ]] || {
    echo "  ${1}" >&2
  }

  os::log::stack $stack_skip

  echo "Exiting with status ${code}" >&2
  exit "${code}"
}

# Log an error but keep going.  Don't dump the stack or exit.
os::log::error() {
  echo "!!! ${1-}" >&2
  shift
  for message; do
    echo "    $message" >&2
  done
}

# Print an usage message to stderr.  The arguments are printed directly.
os::log::usage() {
  echo >&2
  local message
  for message; do
    echo "$message" >&2
  done
  echo >&2
}

os::log::usage_from_stdin() {
  local messages=()
  while read -r line; do
    messages+=$line
  done

  os::log::usage "${messages[@]}"
}

# Print out some info that isn't a top level status line
os::log::info() {
  for message; do
    echo "$message"
  done
}

os::log::info_from_stdin() {
  local messages=()
  while read -r line; do
    messages+=$line
  done

  os::log::info "${messages[@]}"
}

# Print a status line.  Formatted to show up in a stream of output.
os::log::status() {
  echo "+++ $1"
  shift
  for message; do
    echo "    $message"
  done
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
