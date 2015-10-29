#!/bin/bash

# This script runs the networking e2e tests. It has 2 modes of operation:
#
#  - ci: for each plugin, deploy a dind cluster, run tests, and teardown
#  - ad-hoc: run tests against an existing cluster
#
# When invoked without arguments, ci mode will be used.  ci mode
# modifies the host on which it is run.  For details please see the
# documentation in hack/dind-cluster.sh.
#
# If one (and only one) of the following arguments is supplied, ad-hoc
# mode will be used:
#
# -c [config root]   specify the config root (parent of openshift.local.config)
#                    of the cluster to target.
#
# -d                 target the default dind cluster.  Target non-default dind
#                    clusters by specifying the appropriate config root instead
#                    (e.g.-c /tmp/openshift-dind-cluster/foo).
#
# -e                 target existing vm-based cluster (whose config root is the
#                    root of the repo).

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

CLUSTER_CMD="bash -x ${OS_ROOT}/hack/dind-cluster.sh"

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

function save-container-logs() {
  local base_dest_dir=$1

  for container_name in "${CONTAINER_NAMES[@]}"; do
    local dest_dir="${base_dest_dir}/${container_name}"
    if [ ! -d "${dest_dir}" ]; then
      mkdir -p "${dest_dir}"
    fi
    container_log_file=/tmp/systemd.log.gz
    sudo docker exec -t "${container_name}" bash -c "journalctl -xe | \
gzip > ${container_log_file}"
    sudo docker cp "${container_name}:${container_log_file}" "${dest_dir}"
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

  run-extended-tests "${OPENSHIFT_CONFIG_ROOT}" "${NETWORKING_E2E_FOCUS}" \
    "${NETWORKING_E2E_SKIP}" "${log_dir}/test.log"
  local exit_status=$?

  set -e

  if [ "${exit_status}" != "0" ]; then
    TEST_FAILURES=$((TEST_FAILURES + 1))
    os::log::error "e2e tests failed for plugin: ${plugin}"
  fi

  os::log::info "Saving container logs"
  save-container-logs "${log_dir}"

  os::log::info "Shutting down docker-in-docker cluster for the ${name} plugin"
  ${CLUSTER_CMD} stop
  DIND_CLEANUP_REQUIRED=0
  rmdir "${OPENSHIFT_CONFIG_ROOT}"
}

function run-extended-tests() {
  local config_root=$1
  local focus_regex=${2:-.etworking[:]*}
  local skip_regex=${3:-}
  local log_path=${4:-}

  if [ -z "${skip_regex}" ]; then
      # The intra-pod test is currently broken for origin.
      skip_regex='Networking.*intra-pod'
      local conf_path="${config_root}/openshift.local.config"
      # Only the multitenant plugin can pass the isolation test
      if ! grep -q 'redhat/openshift-ovs-multitenant' \
           $(find "${conf_path}" -name 'node-config.yaml' | head -n 1); then
        skip_regex="(${skip_regex}|networking: isolation)"
      fi
  fi

  os::util::run-extended-tests "${config_root}" "${focus_regex}" \
    networking.test "${skip_regex}" "${log_path}"
}

CONFIG_ROOT=
# Parse optional arguments to set CONFIG_ROOT
while getopts ":c:de" opt; do
  case $opt in
    c)
      CONFIG_ROOT="${OPTARG}"
      ;;
    d)
      CONFIG_ROOT="/tmp/openshift-dind-cluster/\
${OPENSHIFT_INSTANCE_PREFIX:-openshift}"
      if [ ! -d "${CONFIG_ROOT}" ]; then
        os::log::error "dind cluster not found"
        os::log::info  "To launch a cluster: hack/dind-cluster.sh start"
        exit 1
      fi
      ;;
    e)
      CONFIG_ROOT="${OS_ROOT}"
      ;;
    \?)
      echo "Invalid option: -${OPTARG}" >&2
      exit 1
      ;;
    :)
      echo "Option -${OPTARG} requires an argument." >&2
      exit 1
      ;;
  esac
done

if [ "${CONFIG_ROOT}" != "" ]; then
  CONFIG_FILE="${CONFIG_ROOT}/openshift.local.config/master/admin.kubeconfig"
  if [ ! -f "${CONFIG_FILE}" ]; then
    os::log::error "${CONFIG_FILE} not found"
    exit 1
  fi
fi

os::build::setup_env

os::log::info "Installing ginkgo"
go get github.com/onsi/ginkgo/ginkgo

os::log::info "Building networking test binary"
TEST_BINARY="${OS_OUTPUT_BINPATH}/networking.test"
if [ -f "${TEST_BINARY}" ] &&
   [ "${OPENSHIFT_SKIP_BUILD:-false}" = "true" ]; then
  os::log::warn "Skipping rebuild of test binary due to OPENSHIFT_SKIP_BUILD=true"
else
  go test -c ./test/extended/networking -o "${TEST_BINARY}"
fi

os::log::info "Starting 'networking' extended tests"
if [ "${CONFIG_ROOT}" != "" ]; then
  os::log::info "CONFIG_ROOT=${CONFIG_ROOT}"
  # Run tests against an existing cluster
  run-extended-tests "${CONFIG_ROOT}" "${NETWORKING_E2E_FOCUS}" \
    "${NETWORKING_E2E_SKIP}"
else
  # For each plugin, run tests against a test-managed cluster

  # Use a unique instance prefix to ensure the names of the test dind
  # containers will not clash with the names of non-test containers.
  export OPENSHIFT_INSTANCE_PREFIX="nettest"
  # TODO(marun) Discover these names instead of hard-coding
  CONTAINER_NAMES=(
    "${OPENSHIFT_INSTANCE_PREFIX}-master"
    "${OPENSHIFT_INSTANCE_PREFIX}-node-1"
    "${OPENSHIFT_INSTANCE_PREFIX}-node-2"
  )

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

  # Avoid unnecessary go builds for subsequent deployments
  export OPENSHIFT_SKIP_BUILD=true

  test-osdn-plugin "multitenant" "redhat/openshift-ovs-multitenant"
fi
