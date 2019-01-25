#!/usr/bin/env bash

set -euo pipefail

base=$(dirname "${BASH_SOURCE[0]}")

# Controls verbosity of the script output and logging.
KUBE_VERBOSE="${KUBE_VERBOSE:-5}"

# A set of helpers for starting/running etcd for tests

ETCD_VERSION=${ETCD_VERSION:-3.2.16}
ETCD_HOST=${ETCD_HOST:-127.0.0.1}
ETCD_PORT=${ETCD_PORT:-2379}
export KUBE_INTEGRATION_ETCD_URL="https://${ETCD_HOST}:${ETCD_PORT}"

local::master::etcd::validate() {
  # validate if in path
  command -v etcd >/dev/null || {
    local::master::log::usage "etcd must be in your PATH"
    local::master::log::info "You can use 'hack/install-etcd.sh' to install a copy in third_party/."
    exit 1
  }

  # validate etcd port is free
  local port_check_command
  if command -v ss &> /dev/null && ss -Version | grep 'iproute2' &> /dev/null; then
    port_check_command="ss"
  elif command -v netstat &>/dev/null; then
    port_check_command="netstat"
  else
    local::master::log::usage "unable to identify if etcd is bound to port ${ETCD_PORT}. unable to find ss or netstat utilities."
    exit 1
  fi
  if ${port_check_command} -nat | grep "LISTEN" | grep "[\.:]${ETCD_PORT:?}" >/dev/null 2>&1; then
    local::master::log::usage "unable to start etcd as port ${ETCD_PORT} is in use. please stop the process listening on this port and retry."
    local::master::log::usage "`netstat -nat | grep "[\.:]${ETCD_PORT:?} .*LISTEN"`"
    exit 1
  fi

  # validate installed version is at least equal to minimum
  version=$(etcd --version | tail -n +1 | head -n 1 | cut -d " " -f 3)
  if [[ $(local::master::etcd::version $ETCD_VERSION) -gt $(local::master::etcd::version $version) ]]; then
   hash etcd
   echo $PATH
   version=$(etcd --version | head -n 1 | cut -d " " -f 3)
   if [[ $(local::master::etcd::version $ETCD_VERSION) -gt $(local::master::etcd::version $version) ]]; then
    local::master::log::usage "etcd version ${ETCD_VERSION} or greater required."
    exit 1
   fi
  fi
}

local::master::etcd::version() {
  printf '%s\n' "${@}" | awk -F . '{ printf("%d%03d%03d\n", $1, $2, $3) }'
}

local::master::etcd::start() {
  # validate before running
  local::master::etcd::validate

  # Start etcd
  ETCD_DIR=${ETCD_DIR:-$(mktemp -d 2>/dev/null || mktemp -d -t test-etcd.XXXXXX)}
  if [[ -d "${ARTIFACTS_DIR:-}" ]]; then
    ETCD_LOGFILE="${ARTIFACTS_DIR}/etcd.$(uname -n).$(id -un).log.DEBUG.$(date +%Y%m%d-%H%M%S).$$"
  else
    ETCD_LOGFILE=${ETCD_LOGFILE:-"/dev/null"}
  fi
  local::master::log::info "etcd --advertise-client-urls ${KUBE_INTEGRATION_ETCD_URL} --data-dir ${ETCD_DIR}/data --listen-client-urls http://${ETCD_HOST}:${ETCD_PORT} --debug > \"${ETCD_LOGFILE}\" 2>/dev/null"
  etcd --advertise-client-urls ${KUBE_INTEGRATION_ETCD_URL} --cert-file=${ETCD_DIR}/serving-etcd-server.crt --key-file=${ETCD_DIR}/serving-etcd-server.key --data-dir ${ETCD_DIR}/data --listen-client-urls ${KUBE_INTEGRATION_ETCD_URL} --debug 2> "${ETCD_LOGFILE}" >/dev/null &
  ETCD_PID=$!

  echo "Waiting for etcd to come up."
  local::master::util::wait_for_url "${KUBE_INTEGRATION_ETCD_URL}/version" "etcdserver" 0.25 80
}

local::master::etcd::stop() {
  if [[ -n "${ETCD_PID-}" ]]; then
    kill "${ETCD_PID}" &>/dev/null || :
    wait "${ETCD_PID}" &>/dev/null || :
  fi
}

local::master::etcd::clean_etcd_dir() {
  if [[ -n "${ETCD_DIR-}" ]]; then
    rm -rf "${ETCD_DIR}/data"
  fi
}

local::master::etcd::cleanup() {
  local::master::etcd::stop
  local::master::etcd::clean_etcd_dir
}

local::master::util::sortable_date() {
  date "+%Y%m%d-%H%M%S"
}

# arguments: target, item1, item2, item3, ...
# returns 0 if target is in the given items, 1 otherwise.
local::master::util::array_contains() {
  local search="$1"
  local element
  shift
  for element; do
    if [[ "${element}" == "${search}" ]]; then
      return 0
     fi
  done
  return 1
}

local::master::util::wait_for_url() {
  local url=$1
  local prefix=${2:-}
  local wait=${3:-1}
  local times=${4:-30}
  local maxtime=${5:-1}

  which curl >/dev/null || {
    local::master::log::usage "curl must be installed"
    exit 1
  }

  local i
  for i in $(seq 1 "$times"); do
    local out
    if out=$(curl --max-time "$maxtime" -gkfs "$url" 2>/dev/null); then
      local::master::log::status "On try ${i}, ${prefix}: ${out}"
      return 0
    fi
    sleep "${wait}"
  done
  local::master::log::error "Timed out waiting for ${prefix} to answer at ${url}; tried ${times} waiting ${wait} between each"
  return 1
}

# Example:  local::master::util::trap_add 'echo "in trap DEBUG"' DEBUG
# See: http://stackoverflow.com/questions/3338030/multiple-bash-traps-for-the-same-signal
local::master::util::trap_add() {
  local trap_add_cmd
  trap_add_cmd=$1
  shift

  for trap_add_name in "$@"; do
    local existing_cmd
    local new_cmd

    # Grab the currently defined trap commands for this trap
    existing_cmd=`trap -p "${trap_add_name}" |  awk -F"'" '{print $2}'`

    if [[ -z "${existing_cmd}" ]]; then
      new_cmd="${trap_add_cmd}"
    else
      new_cmd="${trap_add_cmd};${existing_cmd}"
    fi

    # Assign the test
    trap "${new_cmd}" "${trap_add_name}"
  done
}

# Opposite of local::master::util::ensure-temp-dir()
local::master::util::cleanup-temp-dir() {
  if [[ -n "${KUBE_TEMP-}" ]]; then
    rm -rf "${KUBE_TEMP}"
  fi
}

# Create a temp dir that'll be deleted at the end of this bash session.
#
# Vars set:
#   KUBE_TEMP
local::master::util::ensure-temp-dir() {
  if [[ -z ${KUBE_TEMP-} ]]; then
    KUBE_TEMP=$(mktemp -d 2>/dev/null || mktemp -d -t kubernetes.XXXXXX)
  fi
}

# This figures out the host platform without relying on golang.  We need this as
# we don't want a golang install to be a prerequisite to building yet we need
# this info to figure out where the final binaries are placed.
local::master::util::host_platform() {
  local host_os
  local host_arch
  case "$(uname -s)" in
    Darwin)
      host_os=darwin
      ;;
    Linux)
      host_os=linux
      ;;
    *)
      local::master::log::error "Unsupported host OS.  Must be Linux or Mac OS X."
      exit 1
      ;;
  esac

  case "$(uname -m)" in
    x86_64*)
      host_arch=amd64
      ;;
    i?86_64*)
      host_arch=amd64
      ;;
    amd64*)
      host_arch=amd64
      ;;
    aarch64*)
      host_arch=arm64
      ;;
    arm64*)
      host_arch=arm64
      ;;
    arm*)
      host_arch=arm
      ;;
    i?86*)
      host_arch=x86
      ;;
    s390x*)
      host_arch=s390x
      ;;
    ppc64le*)
      host_arch=ppc64le
      ;;
    *)
      local::master::log::error "Unsupported host arch. Must be x86_64, 386, arm, arm64, s390x or ppc64le."
      exit 1
      ;;
  esac
  echo "${host_os}/${host_arch}"
}

# Test whether openssl is installed.
# Sets:
#  OPENSSL_BIN: The path to the openssl binary to use
function local::master::util::test_openssl_installed {
    openssl version >& /dev/null
    if [ "$?" != "0" ]; then
      echo "Failed to run openssl. Please ensure openssl is installed"
      exit 1
    fi

    OPENSSL_BIN=$(command -v openssl)
}

# creates a client CA, args are sudo, dest-dir, ca-id, purpose
# purpose is dropped in after "key encipherment", you usually want
# '"client auth"'
# '"server auth"'
# '"client auth","server auth"'
function local::master::util::create_signing_certkey {
    local sudo=$1
    local dest_dir=$2
    local id=$3
    local purpose=$4
    # Create client ca
    /usr/bin/env bash -e <<EOF
    rm -f "${dest_dir}/${id}-ca.crt" "${dest_dir}/${id}-ca.key"
    ${OPENSSL_BIN} req -x509 -sha256 -new -nodes -days 365 -newkey rsa:2048 -keyout "${dest_dir}/${id}-ca.key" -out "${dest_dir}/${id}-ca.crt" -subj "/C=xx/ST=x/L=x/O=x/OU=x/CN=ca/emailAddress=x/"
    echo '{"signing":{"default":{"expiry":"43800h","usages":["signing","key encipherment",${purpose}]}}}' > "${dest_dir}/${id}-ca-config.json"
EOF
}

# signs a client certificate: args are sudo, dest-dir, CA, filename (roughly), username, groups...
function local::master::util::create_client_certkey {
    local sudo=$1
    local dest_dir=$2
    local ca=$3
    local id=$4
    local cn=${5:-$4}
    local groups=""
    local SEP=""
    shift 5
    while [ -n "${1:-}" ]; do
        groups+="${SEP}{\"O\":\"$1\"}"
        SEP=","
        shift 1
    done
    /usr/bin/env bash -e <<EOF
    cd ${dest_dir}
    echo '{"CN":"${cn}","names":[${groups}],"hosts":[""],"key":{"algo":"rsa","size":2048}}' | ${CFSSL_BIN} gencert -ca=${ca}.crt -ca-key=${ca}.key -config=${ca}-config.json - | ${CFSSLJSON_BIN} -bare client-${id}
    mv "client-${id}-key.pem" "client-${id}.key"
    mv "client-${id}.pem" "client-${id}.crt"
    rm -f "client-${id}.csr"
EOF
}

# signs a serving certificate: args are sudo, dest-dir, ca, filename (roughly), subject, hosts...
function local::master::util::create_serving_certkey {
    local sudo=$1
    local dest_dir=$2
    local ca=$3
    local id=$4
    local cn=${5:-$4}
    local hosts=""
    local SEP=""
    shift 5
    while [ -n "${1:-}" ]; do
        hosts+="${SEP}\"$1\""
        SEP=","
        shift 1
    done
    /usr/bin/env bash -e <<EOF
    cd ${dest_dir}
    echo '{"CN":"${cn}","hosts":[${hosts}],"key":{"algo":"rsa","size":2048}}' | ${CFSSL_BIN} gencert -ca=${ca}.crt -ca-key=${ca}.key -config=${ca}-config.json - | ${CFSSLJSON_BIN} -bare serving-${id}
    mv "serving-${id}-key.pem" "serving-${id}.key"
    mv "serving-${id}.pem" "serving-${id}.crt"
    rm -f "serving-${id}.csr"
EOF
}

# creates a self-contained kubeconfig: args are sudo, dest-dir, ca file, host, port, client id, token(optional)
function local::master::util::write_client_kubeconfig {
    local sudo=$1
    local dest_dir=$2
    local ca_file=$3
    local api_host=$4
    local api_port=$5
    local client_id=$6
    local token=${7:-}
    cat <<EOF | tee "${dest_dir}"/${client_id}.kubeconfig > /dev/null
apiVersion: v1
kind: Config
clusters:
  - cluster:
      certificate-authority: ${ca_file}
      server: https://${api_host}:${api_port}
    name: localhost:8443
users:
  - user:
      token: ${token}
      client-certificate: ${dest_dir}/client-${client_id}.crt
      client-key: ${dest_dir}/client-${client_id}.key
    name: system:admin/localhost:8443
contexts:
  - context:
      cluster: localhost:8443
      user: system:admin/localhost:8443
    name: /localhost:8443/system:admin
current-context: /localhost:8443/system:admin
EOF

    # flatten the kubeconfig files to make them self contained
    username=$(id -u)
    /usr/bin/env bash -e <<EOF
    oc --config="${dest_dir}/${client_id}.kubeconfig" config view --minify --flatten > "/tmp/${client_id}.kubeconfig"
    mv -f "/tmp/${client_id}.kubeconfig" "${dest_dir}/${client_id}.kubeconfig"
    chown ${username} "${dest_dir}/${client_id}.kubeconfig"
EOF
}

# Wait for background jobs to finish. Return with
# an error status if any of the jobs failed.
local::master::util::wait-for-jobs() {
  local fail=0
  local job
  for job in $(jobs -p); do
    wait "${job}" || fail=$((fail + 1))
  done
  return ${fail}
}

# local::master::util::join <delim> <list...>
# Concatenates the list elements with the delimiter passed as first parameter
#
# Ex: local::master::util::join , a b c
#  -> a,b,c
function local::master::util::join {
  local IFS="$1"
  shift
  echo "$*"
}

# Downloads cfssl/cfssljson into $1 directory if they do not already exist in PATH
#
# Assumed vars:
#   $1 (cfssl directory) (optional)
#
# Sets:
#  CFSSL_BIN: The path of the installed cfssl binary
#  CFSSLJSON_BIN: The path of the installed cfssljson binary
#
function local::master::util::ensure-cfssl {
  if command -v cfssl &>/dev/null && command -v cfssljson &>/dev/null; then
    CFSSL_BIN=$(command -v cfssl)
    CFSSLJSON_BIN=$(command -v cfssljson)
    return 0
  fi

  # Create a temp dir for cfssl if no directory was given
  local cfssldir=${1:-}
  if [[ -z "${cfssldir}" ]]; then
    local::master::util::ensure-temp-dir
    cfssldir="${KUBE_TEMP}/cfssl"
  fi

  mkdir -p "${cfssldir}"
  pushd "${cfssldir}" > /dev/null

    echo "Unable to successfully run 'cfssl' from $PATH; downloading instead..."
    kernel=$(uname -s)
    case "${kernel}" in
      Linux)
        curl --retry 10 -L -o cfssl https://pkg.cfssl.org/R1.2/cfssl_linux-amd64
        curl --retry 10 -L -o cfssljson https://pkg.cfssl.org/R1.2/cfssljson_linux-amd64
        ;;
      Darwin)
        curl --retry 10 -L -o cfssl https://pkg.cfssl.org/R1.2/cfssl_darwin-amd64
        curl --retry 10 -L -o cfssljson https://pkg.cfssl.org/R1.2/cfssljson_darwin-amd64
        ;;
      *)
        echo "Unknown, unsupported platform: ${kernel}." >&2
        echo "Supported platforms: Linux, Darwin." >&2
        exit 2
    esac

    chmod +x cfssl || true
    chmod +x cfssljson || true

    CFSSL_BIN="${cfssldir}/cfssl"
    CFSSLJSON_BIN="${cfssldir}/cfssljson"
    if [[ ! -x ${CFSSL_BIN} || ! -x ${CFSSLJSON_BIN} ]]; then
      echo "Failed to download 'cfssl'. Please install cfssl and cfssljson and verify they are in \$PATH."
      echo "Hint: export PATH=\$PATH:\$GOPATH/bin; go get -u github.com/cloudflare/cfssl/cmd/..."
      exit 1
    fi
  popd > /dev/null
}

# local::master::util::ensure_dockerized
# Confirms that the script is being run inside a kube-build image
#
function local::master::util::ensure_dockerized {
  if [[ -f /kube-build-image ]]; then
    return 0
  else
    echo "ERROR: This script is designed to be run inside a kube-build container"
    exit 1
  fi
}

# local::master::util::ensure-gnu-sed
# Determines which sed binary is gnu-sed on linux/darwin
#
# Sets:
#  SED: The name of the gnu-sed binary
#
function local::master::util::ensure-gnu-sed {
  if LANG=C sed --help 2>&1 | grep -q GNU; then
    SED="sed"
  elif which gsed &>/dev/null; then
    SED="gsed"
  else
    local::master::log::error "Failed to find GNU sed as sed or gsed. If you are on Mac: brew install gnu-sed." >&2
    return 1
  fi
}

# Some useful colors.
if [[ -z "${color_start-}" ]]; then
  declare -r color_start="\033["
  declare -r color_red="${color_start}0;31m"
  declare -r color_yellow="${color_start}0;33m"
  declare -r color_green="${color_start}0;32m"
  declare -r color_blue="${color_start}1;34m"
  declare -r color_cyan="${color_start}1;36m"
  declare -r color_norm="${color_start}0m"
fi

# Handler for when we exit automatically on an error.
# Borrowed from https://gist.github.com/ahendrix/7030300
local::master::log::errexit() {
  local err="${PIPESTATUS[@]}"

  # If the shell we are in doesn't have errexit set (common in subshells) then
  # don't dump stacks.
  set +o | grep -qe "-o errexit" || return

  set +o xtrace
  local code="${1:-1}"
  # Print out the stack trace described by $function_stack  
  if [ ${#FUNCNAME[@]} -gt 2 ]
  then
    local::master::log::error "Call tree:"
    for ((i=1;i<${#FUNCNAME[@]}-1;i++))
    do
      local::master::log::error " $i: ${BASH_SOURCE[$i+1]}:${BASH_LINENO[$i]} ${FUNCNAME[$i]}(...)"
    done
  fi  
  local::master::log::error_exit "Error in ${BASH_SOURCE[1]}:${BASH_LINENO[0]}. '${BASH_COMMAND}' exited with status $err" "${1:-1}" 1
}

# Print out the stack trace
#
# Args:
#   $1 The number of stack frames to skip when printing.
local::master::log::stack() {
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
local::master::log::error_exit() {
  local message="${1:-}"
  local code="${2:-1}"
  local stack_skip="${3:-0}"
  stack_skip=$((stack_skip + 1))

  if [[ ${KUBE_VERBOSE} -ge 4 ]]; then
    local source_file=${BASH_SOURCE[$stack_skip]}
    local source_line=${BASH_LINENO[$((stack_skip - 1))]}
    echo "!!! Error in ${source_file}:${source_line}" >&2
    [[ -z ${1-} ]] || {
      echo "  ${1}" >&2
    }

    local::master::log::stack $stack_skip

    echo "Exiting with status ${code}" >&2
  fi

  exit "${code}"
}

# Log an error but keep going.  Don't dump the stack or exit.
local::master::log::error() {
  timestamp=$(date +"[%m%d %H:%M:%S]")
  echo "!!! $timestamp ${1-}" >&2
  shift
  for message; do
    echo "    $message" >&2
  done
}

# Print an usage message to stderr.  The arguments are printed directly.
local::master::log::usage() {
  echo >&2
  local message
  for message; do
    echo "$message" >&2
  done
  echo >&2
}

local::master::log::usage_from_stdin() {
  local messages=()
  while read -r line; do
    messages+=("$line")
  done

  local::master::log::usage "${messages[@]}"
}

# Print out some info that isn't a top level status line
local::master::log::info() {
  local V="${V:-0}"
  if [[ $KUBE_VERBOSE < $V ]]; then
    return
  fi

  for message; do
    echo "$message"
  done
}

# Just like local::master::log::info, but no \n, so you can make a progress bar
local::master::log::progress() {
  for message; do
    echo -e -n "$message"
  done
}

local::master::log::info_from_stdin() {
  local messages=()
  while read -r line; do
    messages+=("$line")
  done

  local::master::log::info "${messages[@]}"
}

# Print a status line.  Formatted to show up in a stream of output.
local::master::log::status() {
  local V="${V:-0}"
  if [[ $KUBE_VERBOSE < $V ]]; then
    return
  fi

  timestamp=$(date +"[%m%d %H:%M:%S]")
  echo "+++ $timestamp $1"
  shift
  for message; do
    echo "    $message"
  done
}

# preserve etcd data. you also need to set ETCD_DIR.
PRESERVE_ETCD="${PRESERVE_ETCD:-false}"
API_PORT=${API_PORT:-8443}
API_SECURE_PORT=${API_SECURE_PORT:-8443}

# WARNING: For DNS to work on most setups you should export API_HOST as the docker0 ip address,
API_HOST=${API_HOST:-localhost}
API_HOST_IP=${API_HOST_IP:-"127.0.0.1"}
ADVERTISE_ADDRESS=${ADVERTISE_ADDRESS:-""}
FIRST_SERVICE_CLUSTER_IP=${FIRST_SERVICE_CLUSTER_IP:-10.0.0.1}
HOSTNAME_OVERRIDE=${HOSTNAME_OVERRIDE:-"127.0.0.1"}
CONTROLPLANE_SUDO=
LOG_LEVEL=${LOG_LEVEL:-3}
# Use to increase verbosity on particular files, e.g. LOG_SPEC=token_controller*=5,other_controller*=4
LOG_SPEC=${LOG_SPEC:-""}
WAIT_FOR_URL_API_SERVER=${WAIT_FOR_URL_API_SERVER:-60}
MAX_TIME_FOR_URL_API_SERVER=${MAX_TIME_FOR_URL_API_SERVER:-1}

function local::master::cleanup() {
  local::master::log::info "Cleaning up..."

  set +e

  # cleanup temp dirs
  local::master::util::cleanup-temp-dir

  jobs -p | xargs -L1 kill 2>/dev/null
  sleep 1
  # etcd requires two sigterms, ensure we get at least a partial shutdown (bug in 3.3.9?)
  jobs -p | xargs -L1 kill 2>/dev/null
  wait

  local::master::ensure_free_port 10252
  local::master::ensure_free_port 2379

  local::master::log::info "Cleanup complete"
}

function local::master::cleanup_config() {
    rm -rf ${LOCALUP_CONFIG}
}

function local::master::ensure_free_port() {
  # checking for free ports is a convenience to the user
  if ! which nc &>/dev/null; then
    return 0
  fi
  if nc 127.0.0.1 $1 </dev/null 2>/dev/null; then
    local::master::log::error "port $1 already in use"
    return 1
  fi
}

# Check if all processes are still running. Prints a warning once each time
# a process dies unexpectedly.
function local::master::healthcheck() {
  if [[ -n "${KUBE_APISERVER_PID-}" ]] && ! kill -0 ${KUBE_APISERVER_PID} 2>/dev/null; then
    local::master::log::error "API server terminated unexpectedly, see ${KUBE_APISERVER_LOG}"
    KUBE_APISERVER_PID=
  fi

  if [[ -n "${KUBE_CONTROLLER_MANAGER_PID-}" ]] && ! kill -0 ${KUBE_CONTROLLER_MANAGER_PID} 2>/dev/null; then
    local::master::log::error "kube-controller-manager terminated unexpectedly, see ${KUBE_CONTROLLER_MANAGER_LOG}"
    KUBE_CONTROLLER_MANAGER_PID=
  fi

  if [[ -n "${OPENSHIFT_APISERVER_PID-}" ]] && ! kill -0 ${OPENSHIFT_APISERVER_PID} 2>/dev/null; then
    local::master::log::error "API server terminated unexpectedly, see ${OPENSHIFT_APISERVER_LOG}"
    OPENSHIFT_APISERVER_PID=
  fi

  if [[ -n "${OPENSHIFT_CONTROLLER_MANAGER_PID-}" ]] && ! kill -0 ${OPENSHIFT_CONTROLLER_MANAGER_PID} 2>/dev/null; then
    local::master::log::error "kube-controller-manager terminated unexpectedly, see ${OPENSHIFT_CONTROLLER_MANAGER_LOG}"
    OPENSHIFT_CONTROLLER_MANAGER_PID=
  fi


  if [[ -n "${ETCD_PID-}" ]] && ! kill -0 ${ETCD_PID} 2>/dev/null; then
    local::master::log::error "etcd terminated unexpectedly"
    ETCD_PID=
  fi
}

function local::master::generate_etcd_certs() {
    # Create CA signers
    local::master::util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${ETCD_DIR}" server '"client auth","server auth"'
    cp "${ETCD_DIR}/server-ca.key" "${ETCD_DIR}/client-ca.key"
    cp "${ETCD_DIR}/server-ca.crt" "${ETCD_DIR}/client-ca.crt"
    cp "${ETCD_DIR}/server-ca-config.json" "${ETCD_DIR}/client-ca-config.json"

    # Create client certs signed with client-ca, given id, given CN and a number of groups
    local::master::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${ETCD_DIR}" 'client-ca' etcd-client etcd-clients

    # Create matching certificates for kube-aggregator
    local::master::util::create_serving_certkey "${CONTROLPLANE_SUDO}" "${ETCD_DIR}" "server-ca" etcd-server "localhost" "127.0.0.1" ${API_HOST_IP}
}

function local::master::generate_kubeapiserver_certs() {
    openssl genrsa -out "${CERT_DIR}/service-account" 2048 2>/dev/null

    # Create CA signers
    local::master::util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" server '"client auth","server auth"'
    cp "${CERT_DIR}/server-ca.key" "${CERT_DIR}/client-ca.key"
    cp "${CERT_DIR}/server-ca.crt" "${CERT_DIR}/client-ca.crt"
    cp "${CERT_DIR}/server-ca-config.json" "${CERT_DIR}/client-ca-config.json"

    # Create auth proxy client ca
    local::master::util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" request-header '"client auth"'

    # serving cert for kube-apiserver
    local::master::util::create_serving_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "server-ca" kube-apiserver kubernetes.default kubernetes.default.svc "localhost" ${API_HOST_IP} ${API_HOST} ${FIRST_SERVICE_CLUSTER_IP}

    # Create client certs signed with client-ca, given id, given CN and a number of groups
    local::master::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' kubelet system:node:${HOSTNAME_OVERRIDE} system:nodes
    local::master::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' controller system:kube-controller-manager
    local::master::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' admin system:admin system:masters
    local::master::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' openshift-apiserver openshift-apiserver system:masters
    local::master::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' openshift-controller-manager openshift-controller-manager system:masters

    # Create matching certificates for kube-aggregator
    local::master::util::create_serving_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "server-ca" kube-aggregator api.kube-public.svc "localhost" ${API_HOST_IP}
    local::master::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" request-header-ca auth-proxy system:auth-proxy
    # TODO remove masters and add rolebinding
    local::master::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' kube-aggregator system:kube-aggregator system:masters
    local::master::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "${ROOT_CA_FILE}" "${API_HOST}" "${API_SECURE_PORT}" kube-aggregator

    cp ${ETCD_DIR}/server-ca.crt ${CERT_DIR}/etcd-serving-ca.crt
    cp ${ETCD_DIR}/client-etcd-client.crt ${CERT_DIR}/client-etcd-client.crt
    cp ${ETCD_DIR}/client-etcd-client.key ${CERT_DIR}/client-etcd-client.key
}

function local::master::generate_kubecontrollermanager_certs() {
    cp ${LOCALUP_CONFIG}/kube-apiserver/service-account ${LOCALUP_CONFIG}/kube-controller-manager/etcd-serving-ca.crt
    cp ${LOCALUP_CONFIG}/kube-apiserver/client-controller.crt ${LOCALUP_CONFIG}/kube-controller-manager/client-controller.crt
    cp ${LOCALUP_CONFIG}/kube-apiserver/client-controller.key ${LOCALUP_CONFIG}/kube-controller-manager/client-controller.key
    local::master::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${LOCALUP_CONFIG}/kube-controller-manager" "${ROOT_CA_FILE}" "${API_HOST}" "${API_SECURE_PORT}" controller
}

function local::master::generate_openshiftapiserver_certs() {
    # Create CA signers
    local::master::util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${LOCALUP_CONFIG}/openshift-apiserver" server '"client auth","server auth"'

    # serving cert for kube-apiserver
    local::master::util::create_serving_certkey "${CONTROLPLANE_SUDO}" "${LOCALUP_CONFIG}/openshift-apiserver" "server-ca" openshift-apiserver openshift.default openshift.default.svc "localhost" ${API_HOST_IP} ${API_HOST} ${FIRST_SERVICE_CLUSTER_IP}

    cp ${LOCALUP_CONFIG}/kube-apiserver/client-openshift-apiserver.crt ${LOCALUP_CONFIG}/openshift-apiserver/client-openshift-apiserver.crt
    cp ${LOCALUP_CONFIG}/kube-apiserver/client-openshift-apiserver.key ${LOCALUP_CONFIG}/openshift-apiserver/client-openshift-apiserver.key
    local::master::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${LOCALUP_CONFIG}/openshift-apiserver" "${ROOT_CA_FILE}" "${API_HOST}" "${API_SECURE_PORT}" openshift-apiserver

    cp ${ETCD_DIR}/server-ca.crt ${LOCALUP_CONFIG}/openshift-apiserver/etcd-serving-ca.crt
    cp ${ETCD_DIR}/client-etcd-client.crt ${LOCALUP_CONFIG}/openshift-apiserver/client-etcd-client.crt
    cp ${ETCD_DIR}/client-etcd-client.key ${LOCALUP_CONFIG}/openshift-apiserver/client-etcd-client.key
}

function local::master::generate_openshiftcontrollermanager_certs() {
    # Create CA signers
    local::master::util::create_signing_certkey "${CONTROLPLANE_SUDO}" "${LOCALUP_CONFIG}/openshift-controller-manager" server '"client auth","server auth"'

    # serving cert for kube-apiserver
    local::master::util::create_serving_certkey "${CONTROLPLANE_SUDO}" "${LOCALUP_CONFIG}/openshift-controller-manager" "server-ca" openshift-controller-manager openshift.default openshift.default.svc "localhost" ${API_HOST_IP} ${API_HOST} ${FIRST_SERVICE_CLUSTER_IP}

    cp ${LOCALUP_CONFIG}/kube-apiserver/client-ca.crt ${LOCALUP_CONFIG}/openshift-controller-manager/client-ca.crt
    cp ${LOCALUP_CONFIG}/kube-apiserver/client-openshift-controller-manager.crt ${LOCALUP_CONFIG}/openshift-controller-manager/client-openshift-controller-manager.crt
    cp ${LOCALUP_CONFIG}/kube-apiserver/client-openshift-controller-manager.key ${LOCALUP_CONFIG}/openshift-controller-manager/client-openshift-controller-manager.key
    local::master::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${LOCALUP_CONFIG}/openshift-controller-manager" "${ROOT_CA_FILE}" "${API_HOST}" "${API_SECURE_PORT}" openshift-controller-manager
}

function local::master::start_etcd() {
    if [ ! -d "${LOCALUP_CONFIG}/etcd" ]; then
        mkdir -p ${LOCALUP_CONFIG}/etcd
        local::master::generate_etcd_certs
    fi
    local::master::log::info "Starting etcd"
    ETCD_LOGFILE=${LOG_DIR}/etcd.log
    local::master::etcd::start
}

function local::master::start_kubeapiserver() {
    if [ ! -d "${LOCALUP_CONFIG}/kube-apiserver" ]; then
        mkdir -p ${LOCALUP_CONFIG}/kube-apiserver
        cp ${base}/kube-apiserver.yaml ${LOCALUP_CONFIG}/kube-apiserver
        local::master::generate_kubeapiserver_certs
    fi

    KUBE_APISERVER_LOG=${LOG_DIR}/kube-apiserver.log
    hypershift openshift-kube-apiserver \
      --v=${LOG_LEVEL} \
      --vmodule="${LOG_SPEC}" \
      --config=${LOCALUP_CONFIG}/kube-apiserver/kube-apiserver.yaml >"${KUBE_APISERVER_LOG}" 2>&1 &
    KUBE_APISERVER_PID=$!

    # Wait for kube-apiserver to come up before launching the rest of the components.
    local::master::log::info "Waiting for kube-apiserver to come up"
    local::master::util::wait_for_url "https://${API_HOST_IP}:${API_SECURE_PORT}/healthz" "kube-apiserver: " 1 ${WAIT_FOR_URL_API_SERVER} ${MAX_TIME_FOR_URL_API_SERVER} \
        || { local::master::log::error "check kube-apiserver logs: ${KUBE_APISERVER_LOG}" ; exit 1 ; }

    # Create kubeconfigs for all components, using client certs
    local::master::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "${ROOT_CA_FILE}" "${API_HOST}" "${API_SECURE_PORT}" admin
    chown "${USER:-$(id -u)}" "${CERT_DIR}/client-admin.key" # make readable for kubectl
}

function local::master::start_kubecontrollermanager() {
    if [ ! -d "${LOCALUP_CONFIG}/kube-controller-manager" ]; then
        mkdir -p ${LOCALUP_CONFIG}/kube-controller-manager
        local::master::generate_kubecontrollermanager_certs
    fi

    KUBE_CONTROLLER_MANAGER_LOG=${LOG_DIR}/kube-controller-manager.log
    hyperkube controller-manager \
      --v=${LOG_LEVEL} \
      --vmodule="${LOG_SPEC}" \
      --cert-dir="${CERT_DIR}" \
      --service-account-private-key-file="${LOCALUP_CONFIG}/kube-controller-manager/etcd-serving-ca.crt" \
      --root-ca-file="${ROOT_CA_FILE}" \
      --kubeconfig  ${LOCALUP_CONFIG}/kube-controller-manager/controller.kubeconfig \
      --use-service-account-credentials \
      --leader-elect=false >"${KUBE_CONTROLLER_MANAGER_LOG}" 2>&1 &
    KUBE_CONTROLLER_MANAGER_PID=$!

    local::master::log::info "Waiting for kube-controller-manager to come up"
    local::master::util::wait_for_url "http://localhost:10252/healthz" "kube-controller-manager: " 1 ${WAIT_FOR_URL_API_SERVER} ${MAX_TIME_FOR_URL_API_SERVER} \
        || { local::master::log::error "check kube-controller-manager logs: ${KUBE_CONTROLLER_MANAGER_LOG}" ; exit 1 ; }
}

function local::master::start_openshiftapiserver() {
    if [ ! -d "${LOCALUP_CONFIG}/openshift-apiserver" ]; then
        mkdir -p ${LOCALUP_CONFIG}/openshift-apiserver
        cp ${base}/openshift-apiserver.yaml ${LOCALUP_CONFIG}/openshift-apiserver
        local::master::generate_openshiftapiserver_certs
    fi

    OPENSHIFT_APISERVER_LOG=${LOG_DIR}/openshift-apiserver.log
    hypershift openshift-apiserver \
      --v=${LOG_LEVEL} \
      --vmodule="${LOG_SPEC}" \
      --config=${LOCALUP_CONFIG}/openshift-apiserver/openshift-apiserver.yaml >"${OPENSHIFT_APISERVER_LOG}" 2>&1 &
    OPENSHIFT_APISERVER_PID=$!

    # Wait for openshift-apiserver to come up before launching the rest of the components.
    local::master::log::info "Waiting for openshift-apiserver to come up"
    local::master::util::wait_for_url "https://${API_HOST_IP}:8444/healthz" "openshift-apiserver: " 1 ${WAIT_FOR_URL_API_SERVER} ${MAX_TIME_FOR_URL_API_SERVER} \
        || { local::master::log::error "check kube-apiserver logs: ${OPENSHIFT_APISERVER_LOG}" ; exit 1 ; }

    # the apiservice requires an endpoint, which may not be a loopback address
    public_address=${API_HOST_IP:-}
    if [[ -z "${public_address}" || "${public_address}" == "127.0.0.1" ]]; then
      if which ifconfig 2>/dev/null; then
        public_address=$( ifconfig | sed -En 's/127.0.0.1//;s/.*inet (addr:)?(([0-9]*\.){3}[0-9]*).*/\2/p' | head -1 )
      elif which ip 2>/dev/null; then
        public_address=$(ip -o -4 addr show up primary scope global | awk '{print $4}' | cut -f1 -d'/' | head -n1)
      else
        local:master::log::error "Unable to find public address, set API_HOST_IP to a non-loopback address"
        exit 1
      fi
    fi
    for filename in ${base}/apiservice-*.yaml; do
        sed "s/NON_LOOPBACK_HOST/${public_address}/g" ${filename} | oc --config=${LOCALUP_CONFIG}/openshift-apiserver/openshift-apiserver.kubeconfig apply -f -
    done
}

function local::master::start_openshiftcontrollermanager() {
    mkdir -p ${LOCALUP_CONFIG}/openshift-controller-manager
    cp ${base}/openshift-controller-manager.yaml ${LOCALUP_CONFIG}/openshift-controller-manager
    local::master::generate_openshiftcontrollermanager_certs

    OPENSHIFT_CONTROLLER_MANAGER_LOG=${LOG_DIR}/openshift-controller-manager.log
    hypershift openshift-controller-manager \
      --v=${LOG_LEVEL} \
      --vmodule="${LOG_SPEC}" \
      --config=${LOCALUP_CONFIG}/openshift-controller-manager/openshift-controller-manager.yaml >"${OPENSHIFT_CONTROLLER_MANAGER_LOG}" 2>&1 &
    OPENSHIFT_CONTROLLER_MANAGER_PID=$!

    local::master::log::info "Waiting for openshift-controller-manager to come up"
    local::master::util::wait_for_url "https://localhost:8445/healthz" "openshift-controller-manager: " 1 ${WAIT_FOR_URL_API_SERVER} ${MAX_TIME_FOR_URL_API_SERVER} \
        || { local::master::log::error "check openshift-controller-manager logs: ${OPENSHIFT_CONTROLLER_MANAGER_LOG}" ; exit 1 ; }
}

function local::master::init_master() {
    ETCD_DIR=${LOCALUP_CONFIG}/etcd
    CERT_DIR=${LOCALUP_CONFIG}/kube-apiserver
    ROOT_CA_FILE=${CERT_DIR}/server-ca.crt

    # ensure necessary ports are free
    local::master::ensure_free_port 2379
    local::master::ensure_free_port 8443
    local::master::ensure_free_port 8444
    local::master::ensure_free_port 8445
    local::master::ensure_free_port 10252

    local::master::util::test_openssl_installed
    local::master::util::ensure-cfssl

    local::master::start_etcd
    local::master::start_kubeapiserver
    local::master::start_kubecontrollermanager
    local::master::start_openshiftapiserver
    local::master::start_openshiftcontrollermanager

    cp ${LOCALUP_CONFIG}/kube-apiserver/admin.kubeconfig ${LOCALUP_CONFIG}/admin.kubeconfig
    local::master::log::info "Created config directory in ${LOCALUP_CONFIG}"
}

trap 'exit 0' TERM
trap "local::master::cleanup" EXIT

local::master::util::ensure-temp-dir

root=${KUBE_TEMP:-$(pwd)}
export LOCALUP_CONFIG=${1:-${root}}
export LOG_DIR=${LOCALUP_CONFIG}/logs
mkdir -p ${LOG_DIR}

# prevent Kube controller manager from being able to use in-cluster config
unset KUBERNETES_SERVICE_HOST

if [[ -z "${1-}" ]]; then
  echo "Logging to ${LOG_DIR}"
fi

local::master::init_master

echo
echo "Cluster is available, use the following kubeconfig to interact with it"
echo "export KUBECONFIG=${LOCALUP_CONFIG}/admin.kubeconfig"
echo "Press ctrl+C to finish"

while true; do sleep 1; local::master::healthcheck; done
