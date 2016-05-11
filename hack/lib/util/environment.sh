#!/bin/bash

# This script holds library functions for setting up the shell environment for OpenShift scripts
#
# This script assumes $OS_ROOT is set before being sourced
source "${OS_ROOT}/hack/util.sh"

# os::util::environment::use_sudo updates $USE_SUDO to be 'true', so that later scripts choosing between
# execution using 'sudo' and execution without it chose to use 'sudo'
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  - export USE_SUDO
function os::util::environment::use_sudo() {
    export USE_SUDO=true
}

# os::util::environment::setup_time_vars sets up environment variables that describe durations of time
# These variables can be used to specify times for other utility functions
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  - export TIME_MS
#  - export TIME_SEC
#  - export TIME_MIN
function os::util::environment::setup_time_vars() {
    export TIME_MS=1
    export TIME_SEC="$(( 1000  * ${TIME_MS} ))"
    export TIME_MIN="$(( 60 * ${TIME_SEC} ))"
}

# os::util::environment::setup_all_server_vars sets up all environment variables necessary to configure and start an OpenShift server
#
# Globals:
#  - OS_ROOT
#  - PATH
#  - TMPDIR
#  - LOG_DIR
#  - ARTIFACT_DIR
#  - KUBELET_SCHEME
#  - KUBELET_BIND_HOST
#  - KUBELET_HOST
#  - KUBELET_PORT
#  - BASETMPDIR
#  - ETCD_PORT
#  - ETCD_PEER_PORT
#  - API_BIND_HOST
#  - API_HOST
#  - API_PORT
#  - API_SCHEME
#  - PUBLIC_MASTER_HOST
#  - USE_IMAGES
# Arguments:
#  - 1: the path under the root temporary directory for OpenShift where these subdirectories should be made
# Returns:
#  - export PATH
#  - export BASETMPDIR
#  - export LOG_DIR
#  - export VOLUME_DIR
#  - export ARTIFACT_DIR
#  - export FAKE_HOME_DIR
#  - export HOME
#  - export KUBELET_SCHEME
#  - export KUBELET_BIND_HOST
#  - export KUBELET_HOST
#  - export KUBELET_PORT
#  - export ETCD_PORT
#  - export ETCD_PEER_PORT
#  - export ETCD_DATA_DIR
#  - export API_BIND_HOST
#  - export API_HOST
#  - export API_PORT
#  - export API_SCHEME
#  - export CURL_CA_BUNDLE
#  - export CURL_CERT
#  - export CURL_KEY
#  - export SERVER_CONFIG_DIR
#  - export MASTER_CONFIG_DIR
#  - export NODE_CONFIG_DIR
#  - export USE_IMAGES
#  - export TAG
function os::util::environment::setup_all_server_vars() {
    local subtempdir=$1

    os::util::environment::update_path_var
    os::util::environment::setup_tmpdir_vars "${subtempdir}"
    os::util::environment::setup_kubelet_vars
    os::util::environment::setup_etcd_vars
    os::util::environment::setup_server_vars
    os::util::environment::setup_images_vars
}

# os::util::environment::update_path_var updates $PATH so that OpenShift binaries are available
#
# Globals:
#  - OS_ROOT
#  - PATH
# Arguments:
#  None
# Returns:
#  - export PATH
function os::util::environment::update_path_var() {
    export PATH="${OS_ROOT}/_output/local/bin/$(os::util::host_platform):${PATH}"
}

# os::util::environment::setup_misc_tmpdir_vars sets up temporary directory path variables
#
# Globals:
#  - TMPDIR
#  - LOG_DIR
#  - ARTIFACT_DIR
# Arguments:
#  - 1: the path under the root temporary directory for OpenShift where these subdirectories should be made
# Returns:
#  - export BASETMPDIR
#  - export LOG_DIR
#  - export VOLUME_DIR
#  - export ARTIFACT_DIR
#  - export FAKE_HOME_DIR
#  - export HOME
function os::util::environment::setup_tmpdir_vars() {
    local sub_dir=$1

    export BASETMPDIR="${TPMDIR:-/tmp}/openshift/${sub_dir}"
    export LOG_DIR="${LOG_DIR:-${BASETMPDIR}/logs}"
    export VOLUME_DIR="${BASETMPDIR}/volumes"
    export ARTIFACT_DIR="${ARTIFACT_DIR:-${BASETMPDIR}/artifacts}"

    # change the location of $HOME so no one does anything naughty
    export FAKE_HOME_DIR="${BASETMPDIR}/openshift.local.home"
    export HOME="${FAKE_HOME_DIR}"

    mkdir -p  "${BASETMPDIR}" "${LOG_DIR}" "${VOLUME_DIR}" "${ARTIFACT_DIR}" "${HOME}"
}

# os::util::environment::setup_kubelet_vars sets up environment variables necessary for interacting with the kubelet
#
# Globals:
#  - KUBELET_SCHEME
#  - KUBELET_BIND_HOST
#  - KUBELET_HOST
#  - KUBELET_PORT
# Arguments:
#  None
# Returns:
#  - export KUBELET_SCHEME
#  - export KUBELET_BIND_HOST
#  - export KUBELET_HOST
#  - export KUBELET_PORT
function os::util::environment::setup_kubelet_vars() {
    export KUBELET_SCHEME="${KUBELET_SCHEME:-https}"
    export KUBELET_BIND_HOST="${KUBELET_BIND_HOST:-$(openshift start --print-ip)}"
    export KUBELET_HOST="${KUBELET_HOST:-${KUBELET_BIND_HOST}}"
    export KUBELET_PORT="${KUBELET_PORT:-10250}"
}

# os::util::environment::setup_etcd_vars sets up environment variables necessary for interacting with etcd
#
# Globals:
#  - BASETMPDIR
#  - ETCD_HOST
#  - ETCD_PORT
#  - ETCD_PEER_PORT
# Arguments:
#  None
# Returns:
#  - export ETCD_HOST
#  - export ETCD_PORT
#  - export ETCD_PEER_PORT
#  - export ETCD_DATA_DIR
function os::util::environment::setup_etcd_vars() {
    export ETCD_HOST="${ETCD_HOST:-127.0.0.1}"
    export ETCD_PORT="${ETCD_PORT:-4001}"
    export ETCD_PEER_PORT="${ETCD_PEER_PORT:-7001}"

    export ETCD_DATA_DIR="${BASETMPDIR}/etcd"

    mkdir -p "${ETCD_DATA_DIR}"
}

# os::util::environment::setup_server_vars sets up environment variables necessary for interacting with the server
# 
# Globals:
#  - BASETMPDIR
#  - KUBELET_HOST
#  - API_BIND_HOST
#  - API_HOST
#  - API_PORT
#  - API_SCHEME
#  - PUBLIC_MASTER_HOST
# Arguments:
#  None
# Returns:
#  - export API_BIND_HOST
#  - export API_HOST
#  - export API_PORT
#  - export API_SCHEME
#  - export CURL_CA_BUNDLE
#  - export CURL_CERT
#  - export CURL_KEY
#  - export SERVER_CONFIG_DIR
#  - export MASTER_CONFIG_DIR
#  - export NODE_CONFIG_DIR
function os::util::environment::setup_server_vars() {
    export API_BIND_HOST="${API_BIND_HOST:-$(openshift start --print-ip)}"
    export API_HOST="${API_HOST:-${API_BIND_HOST}}"
    export API_PORT="${API_PORT:-8443}"
    export API_SCHEME="${API_SCHEME:-https}"

    export MASTER_ADDR="${API_SCHEME}://${API_HOST}:${API_PORT}"
    export PUBLIC_MASTER_HOST="${PUBLIC_MASTER_HOST:-${API_HOST}}"

    export SERVER_CONFIG_DIR="${BASETMPDIR}/openshift.local.config"
    export MASTER_CONFIG_DIR="${SERVER_CONFIG_DIR}/master"
    export NODE_CONFIG_DIR="${SERVER_CONFIG_DIR}/node-${KUBELET_HOST}"

    mkdir -p "${SERVER_CONFIG_DIR}" "${MASTER_CONFIG_DIR}" "${NODE_CONFIG_DIR}"

    if [[ "${API_SCHEME}" == "https" ]]; then
        export CURL_CA_BUNDLE="${MASTER_CONFIG_DIR}/ca.crt"
        export CURL_CERT="${MASTER_CONFIG_DIR}/admin.crt"
        export CURL_KEY="${MASTER_CONFIG_DIR}/admin.key"
    fi
}

# os::util::environment::setup_images_vars sets up environment variables necessary for interacting with release images
#
# Globals:
#  - OS_ROOT
#  - USE_IMAGES
# Arguments:
#  None
# Returns:
#  - export USE_IMAGES
#  - export TAG
function os::util::environment::setup_images_vars() {
    # Use either the latest release built images, or latest.
    if [[ -z "${USE_IMAGES-}" ]]; then
        export TAG='latest'
        export USE_IMAGES='openshift/origin-${component}:latest'

        if [[ -e "${OS_ROOT}/_output/local/releases/.commit" ]]; then
            export TAG="$(cat "${OS_ROOT}/_output/local/releases/.commit")"
            export USE_IMAGES="openshift/origin-\${component}:${TAG}"
        fi
    fi
}
