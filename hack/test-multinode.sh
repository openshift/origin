#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# Ensure that subshells inherit bash settings (specifically xtrace)
export SHELLOPTS

OS_ROOT="${BASH_SOURCE%/*}/.."
OS_ROOT="$(readlink -ev "${OS_ROOT}")"

source "${OS_ROOT}/hack/text.sh"
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/lib/log.sh"
source "${OS_ROOT}/hack/lib/util/environment.sh"
os::log::install_errexit

CLUSTER_CMD="${OS_ROOT}/hack/dind-cluster.sh"
CONFIG_ROOT="${OPENSHIFT_CONFIG_ROOT:-${OS_ROOT}}"

# Use a unique instance prefix to ensure the names of the test dind
# containers will not clash with the names of non-test containers.
export OPENSHIFT_INSTANCE_PREFIX="testing"
export OPENSHIFT_NUM_MINIONS="${OPENSHIFT_NUM_MINIONS:-3}"

# TODO(marun) Discover these names instead of hard-coding
export NODE_MINIONS="$(seq -f "${OPENSHIFT_INSTANCE_PREFIX}-node-%g" 1 ${OPENSHIFT_NUM_MINIONS})"
export NODE_NAMES="$NODE_MINIONS ${OPENSHIFT_INSTANCE_PREFIX}-master"

os::util::environment::setup_tmpdir_vars "test-multinode"
reset_tmp_dir

os::log::start_system_logger

os::log::info "Building docker-in-docker images"
${CLUSTER_CMD} build-images

function unpause-cluster() {
    echo
    os::text::print_bold "Unpausing docker containers if any"
    for container in ${NODE_NAMES}; do
        docker inspect -f '{{.State.Paused}}' "$container" |grep -qsFx false ||
            docker unpause "$container" ||:
    done
}

# Ensure cleanup on error
function cleanup-dind() {
    local rc=$?

    if [ -z "${SKIP_TEARDOWN-}" ]; then
        unpause-cluster
        echo
        os::text::print_bold "Shutting down docker-in-docker cluster"
        "${CLUSTER_CMD}" stop
    fi

    exit $rc
}

trap "exit" HUP PIPE INT QUIT TERM
trap "cleanup-dind" EXIT

find "${OS_ROOT}/test/multinode" -name '*.sh' |
    grep -E "${1:-.*}" |
    sort -u |
while read testfile; do
    echo
    os::log::info "+++ ${testfile}"

    name="${testfile##*/}"
    name="${name%.sh}"

    export OC_PROJECT="${OPENSHIFT_INSTANCE_PREFIX}-${name}"

    unpause-cluster

    echo
    os::text::print_bold "Launching a docker-in-docker cluster for ${name} ..."
    export OPENSHIFT_SKIP_BUILD=true
    export OPENSHIFT_NETWORK_PLUGIN="subnet"
    export OPENSHIFT_CONFIG_ROOT="${BASETMPDIR}/${name}"
    export OS_DIND_BUILD_IMAGES=0

    "${CLUSTER_CMD}" restart
    "${CLUSTER_CMD}" wait-for-cluster

    MASTER_CONFIG_DIR="${BASETMPDIR}/${name}/openshift.local.config/master"

    # Set variables for oc from now on
    export KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
    export PATH="$PATH:${OS_OUTPUT_BINPATH}/$(os::util::host_platform)"

    export CLUSTER_ADMIN_CONTEXT=$(oc config view --config="${KUBECONFIG}" --flatten -o template --template='{{index . "current-context"}}')

    export CURL_CA_BUNDLE="${MASTER_CONFIG_DIR}/ca.crt"
    export CURL_CERT="${MASTER_CONFIG_DIR}/admin.crt"
    export CURL_KEY="${MASTER_CONFIG_DIR}/admin.key"

    os::text::print_bold "Creating a new project '${OC_PROJECT}' ..."
    oc project "${CLUSTER_ADMIN_CONTEXT}"
    oc new-project "${OC_PROJECT}"

    echo
    os::text::print_bold "Running ${testfile} ..."

    rc=
    "${testfile}" || rc=$?

    echo
    echo -n "Test ${testfile} "
    if [[ -n "$rc" ]]; then
        os::text::print_red   "FAILED"
        exit $rc
    else
        os::text::print_green "PASSED"
    fi
    echo

    # Cleanup after testing
    oc delete all --all

    os::text::print_bold "Remove project '${OC_PROJECT}' ..."
    oc project "${CLUSTER_ADMIN_CONTEXT}"
    oc delete project "${OC_PROJECT}"
done
