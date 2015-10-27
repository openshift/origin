#!/bin/bash
#
# WARNING: This script modifies the host on which it is run.  For
# details please see the documentation in hack/dind-cluster.sh
#
# This script runs the openshift sdn end-to-end tests.  It is intended
# to encapsulate the setup, running, and teardown for reuse by both CI
# and developers.
#
# Dependencies: The docker daemon must be installed locally.

set -o errexit
set -o nounset
set -o pipefail
set -o xtrace

OS_ROOT=$(dirname "${BASH_SOURCE}")/../..
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/common.sh"
os::log::install_errexit

# These strings filter the available tests.
NETWORKING_E2E_FOCUS="${NETWORKING_E2E_FOCUS:-}"
NETWORKING_E2E_SKIP="${NETWORKING_E2E_SKIP:-}"

# Use a unique instance prefix to ensure the names of the test dind
# containers will not clash with the names of non-test containers.
export OS_INSTANCE_PREFIX="nettest"
# TODO(marun) Discover these names instead of hard-coding
CONTAINER_NAMES=(
  "${OS_INSTANCE_PREFIX}-master"
  "${OS_INSTANCE_PREFIX}-node-1"
  "${OS_INSTANCE_PREFIX}-node-2"
  )

CLUSTER_CMD=${OS_ROOT}/hack/dind-cluster.sh

# Control variable to limit unnecessary cleanup
DIND_CLEANUP_REQUIRED=0

function copy-container-files() {
  local source_path=$1
  local base_dest_dir=$2

  for container_name in "${CONTAINER_NAMES[@]}"; do
    local dest_dir="${base_dest_dir}/${container_name}"
    if [ ! -d "${dest_dir}" ]; then
      mkdir -p "${dest_dir}"
    fi
    sudo docker cp "${container_name}:${source_path}" "${dest_dir}"
  done
}

function save-artifacts() {
  local name=$1
  local config_root=$2

  local dest_dir="${ARTIFACT_DIR}/${name}"

  local config_source="${config_root}/openshift.local.config"
  local config_dest="${dest_dir}/openshift.local.config"
  mkdir -p "${config_dest}"
  cp -r ${config_source}/* ${config_dest}/

  copy-container-files "/etc/hosts" "${dest_dir}"
}

# Any non-zero exit code from any test run invoked by this script
# should increment TEST_FAILURE so the total count of failed test runs
# can be returned as the exit code.
TEST_FAILURES=0
function test-osdn-plugin() {
  local name=$1
  local plugin=$2

  os::log::info "Targeting ${name} plugin: ${plugin}"
  os::log::info "Launching a docker-in-docker cluster for the ${name} plugin"
  export OPENSHIFT_SDN="${plugin}"
  export OPENSHIFT_CONFIG_ROOT="${BASETMPDIR}/${name}"
  # Images have already been built
  export OS_DIND_BUILD_IMAGES=0
  DIND_CLEANUP_REQUIRED=1
  ${CLUSTER_CMD} start

  os::log::info "Saving cluster configuration"
  save-artifacts "${name}" "${OPENSHIFT_CONFIG_ROOT}"

  os::log::info "Running networking e2e tests against the ${name} plugin"
  local log_dir="${LOG_DIR}/${name}"
  mkdir -p "${log_dir}"

  # Disable error checking for the test run to ensure that failures
  # for one plugin do not prevent a test run against a different
  # plugin.
  set +e

  os::util::run-net-extended-tests "${OPENSHIFT_CONFIG_ROOT}" \
    "${NETWORKING_E2E_FOCUS}" "${NETWORKING_E2E_SKIP}" "${log_dir}/test.log"
  local exit_status=$?

  set -e

  if [ "${exit_status}" != "0" ]; then
    TEST_FAILURES=$((TEST_FAILURES + 1))
  fi

  # TODO(marun) Need to dump logs from systemd
  os::log::info "Saving daemon logs"
  copy-container-files "/var/log/supervisor" "${LOG_DIR}/${name}"

  os::log::info "Shutting down docker-in-docker cluster for the ${name} plugin"
  ${CLUSTER_CMD} stop
  DIND_CLEANUP_REQUIRED=0
  rmdir "${OPENSHIFT_CONFIG_ROOT}"
}

ensure_ginkgo_or_die

os::build::setup_env
go test -c ./test/extended/networking -o ${OS_OUTPUT_BINPATH}/networking.test

os::log::info "Starting 'networking' extended tests"

export TMPDIR="${TMPDIR:-"/tmp"}"
export BASETMPDIR="${TMPDIR}/openshift-extended-tests/networking"
setup_env_vars
reset_tmp_dir

os::log::info "Building docker-in-docker images"
${CLUSTER_CMD} build-images

# Ensure cleanup on error
ENABLE_SELINUX=0
function cleanup-dind {
  local exit_code=$?
  # Return non-zero for either command or test failures
  if [ "${exit_code}" = "0" ]; then
    exit_code="${TEST_FAILURES}"
  fi
  if [ "${DIND_CLEANUP_REQUIRED}" = "1" ]; then
    os::log::info "Shutting down docker-in-docker cluster"
    ${CLUSTER_CMD} stop
  fi
  enable-selinux
  exit $exit_code
}
trap "exit" INT TERM
trap "cleanup-dind" EXIT

# Docker-in-docker is not compatible with selinux
disable-selinux

os::log::info "Ensuring that previous test cluster is shut down"
${CLUSTER_CMD} stop

test-osdn-plugin "subnet" "redhat/openshift-ovs-subnet"
test-osdn-plugin "multitenant" "redhat/openshift-ovs-multitenant"
