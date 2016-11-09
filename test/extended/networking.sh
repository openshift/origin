#!/bin/bash

# This script runs the networking e2e tests. See CONTRIBUTING.adoc for
# documentation.
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"

if [[ -n "${OPENSHIFT_VERBOSE_OUTPUT:-}" ]]; then
  set -o xtrace
  export PS4='+ \D{%b %d %H:%M:%S} $(basename ${BASH_SOURCE}):${LINENO} ${FUNCNAME[0]:+${FUNCNAME[0]}(): }'
fi

# Ensure that subshells inherit bash settings (specifically xtrace)
export SHELLOPTS

# These strings filter the available tests.
#
# The EmptyDir test is a canary; it will fail if mount propagation is
# not properly configured on the host.
NETWORKING_E2E_FOCUS="${NETWORKING_E2E_FOCUS:-etworking|Services should be able to create a functioning NodePort service|EmptyDir volumes should support \(root,0644,tmpfs\)}"
NETWORKING_E2E_SKIP="${NETWORKING_E2E_SKIP:-}"

# Limit the scope of execution to minimize runtime
NETWORKING_E2E_MINIMAL="${NETWORKING_E2E_MINIMAL:-}"

DEFAULT_SKIP_LIST=(
  # TODO(marun) This should work with docker >= 1.10
  "openshift router"
  "\[Feature:Federation\]"

  # Skipped until https://github.com/openshift/origin/issues/11042 is resolved
  "should preserve source pod IP for traffic thru service cluster IP"

  # Panicing, needs investigation
  "Networking IPerf"

  # Skipped due to origin returning 403 for some of the urls
  "should provide unchanging, static URL paths for kubernetes api services"

  # DNS inside container fails in CI but works locally
  "should provide Internet connection for containers"

  # Skip tests that require GCE or AWS. (They'll skip themselves if we run them, but
  # only after several seconds of setup.)
  "should be able to up and down services"
  "should work after restarting kube-proxy"
  "should work after restarting apiserver"
  "should be able to change the type and ports of a service"
)

MINIMAL_SKIP_LIST=(
  "OVS"
)

CLUSTER_CMD="${OS_ROOT}/hack/dind-cluster.sh"

# Control variable to limit unnecessary cleanup
DIND_CLEANUP_REQUIRED=0

function copy-container-files() {
  local source_path=$1
  local base_dest_dir=$2

  for container_name in "${CONTAINER_NAMES[@]}"; do
    local dest_dir="${base_dest_dir}/${container_name}"
    if [[ ! -d "${dest_dir}" ]]; then
      mkdir -p "${dest_dir}"
    fi
    sudo docker cp "${container_name}:${source_path}" "${dest_dir}"
  done
}

function save-container-logs() {
  local base_dest_dir=$1
  local output_to_stdout=${2:-}

  os::log::info "Saving container logs"

  local container_log_file="/tmp/systemd.log.gz"

  for container_name in "${CONTAINER_NAMES[@]}"; do
    local dest_dir="${base_dest_dir}/${container_name}"
    if [[ ! -d "${dest_dir}" ]]; then
      mkdir -p "${dest_dir}"
    fi
    sudo docker exec -t "${container_name}" bash -c "journalctl | \
gzip > ${container_log_file}"
    sudo docker cp "${container_name}:${container_log_file}" "${dest_dir}"
    # Output container logs to stdout to ensure that jenkins has
    # detail to classify the failure cause.
    if [[ -n "${output_to_stdout}" ]]; then
      local msg="System logs for container ${container_name}"
      os::log::info "< ${msg} >"
      os::log::info "***************************************************"
      gunzip --stdout "${dest_dir}/$(basename "${container_log_file}")"
      os::log::info "***************************************************"
      os::log::info "</ ${msg} >"
    fi
  done
}

function save-artifacts() {
  local name=$1
  local config_root=$2

  os::log::info "Saving cluster configuration"

  local dest_dir="${ARTIFACT_DIR}/${name}"

  local config_source="${config_root}/openshift.local.config"
  local config_dest="${dest_dir}/openshift.local.config"
  mkdir -p "${config_dest}"
  cp -r ${config_source}/* ${config_dest}/

  copy-container-files "/etc/hosts" "${dest_dir}"
}

function deploy-cluster() {
  local name=$1
  local plugin=$2
  local log_dir=$3

  os::log::info "Launching a docker-in-docker cluster for the ${name} plugin"
  export OPENSHIFT_CONFIG_ROOT="${BASETMPDIR}/${name}"
  DIND_CLEANUP_REQUIRED=1

  local exit_status=0
  if ! ${CLUSTER_CMD} start -r -n "${plugin}"; then
    exit_status=1
  fi

  save-artifacts "${name}" "${OPENSHIFT_CONFIG_ROOT}"

  return "${exit_status}"
}

function get-kubeconfig-from-root() {
  local config_root=$1

  echo "${config_root}/openshift.local.config/master/admin.kubeconfig"
}

# Any non-zero exit code from any test run invoked by this script
# should increment TEST_FAILURE so the total count of failed test runs
# can be returned as the exit code.
TEST_FAILURES=0
function test-osdn-plugin() {
  local name=$1
  local plugin=$2
  local isolation=$3

  os::log::info "Targeting ${name} plugin: ${plugin}"

  local log_dir="${LOG_DIR}/${name}"
  mkdir -p "${log_dir}"

  local deployment_failed=
  local tests_failed=

  if deploy-cluster "${name}" "${plugin}" "${log_dir}"; then
    os::log::info "Running networking e2e tests against the ${name} plugin"
    export TEST_REPORT_FILE_NAME="${name}-junit"

    local kubeconfig="$(get-kubeconfig-from-root "${OPENSHIFT_CONFIG_ROOT}")"
    if ! TEST_REPORT_FILE_NAME=networking_${name}_${isolation} \
         OPENSHIFT_NETWORK_ISOLATION="${isolation}" \
         run-extended-tests "${kubeconfig}" "${log_dir}/test.log"; then
      tests_failed=1
      os::log::error "e2e tests failed for plugin: ${plugin}"
    fi
  else
    deployment_failed=1
    os::log::error "Failed to deploy cluster for plugin: {$name}"
  fi

  # Record the failure before further errors can occur.
  if [[ -n "${deployment_failed}" || -n "${tests_failed}" ]]; then
    TEST_FAILURES=$((TEST_FAILURES + 1))
  fi

  # Output container logs to stdout if deployment fails
  save-container-logs "${log_dir}" "${deployment_failed}"

  os::log::info "Shutting down docker-in-docker cluster for the ${name} plugin"
  ${CLUSTER_CMD} stop
  DIND_CLEANUP_REQUIRED=0
  rm -rf "${OPENSHIFT_CONFIG_ROOT}"
}


function join { local IFS="$1"; shift; echo "$*"; }

function run-extended-tests() {
  local kubeconfig=$1
  local log_path=${2:-}
  local dlv_debug="${DLV_DEBUG:-}"

  local focus_regex="${NETWORKING_E2E_FOCUS}"
  local skip_regex="${NETWORKING_E2E_SKIP}"

  if [[ -z "${skip_regex}" ]]; then
    skip_regex="$(join '|' "${DEFAULT_SKIP_LIST[@]}")"
    if [[ -n "${NETWORKING_E2E_MINIMAL}" ]]; then
      skip_regex="${skip_regex}|$(join '|' "${MINIMAL_SKIP_LIST[@]}")"
    fi
  fi

  export KUBECONFIG="${kubeconfig}"
  export EXTENDED_TEST_PATH="${OS_ROOT}/test/extended"

  local test_args="--test.v '--ginkgo.skip=${skip_regex}' \
'--ginkgo.focus=${focus_regex}' ${TEST_EXTRA_ARGS}"

  if [[ -n "${dlv_debug}" ]]; then
    # run tests using delve debugger
    local test_cmd="dlv exec ${TEST_BINARY} -- ${test_args}"
  else
    # run tests normally
    local test_cmd="${TEST_BINARY} ${test_args}"
  fi

  if [[ -n "${log_path}" ]]; then
    if [[ -n "${dlv_debug}" ]]; then
      os::log::warn "Not logging to file since DLV_DEBUG is enabled"
    else
      test_cmd="${test_cmd} | tee ${log_path}"
    fi
  fi

  pushd "${EXTENDED_TEST_PATH}/networking" > /dev/null
    eval "${test_cmd}; "'exit_status=${PIPESTATUS[0]}'
  popd > /dev/null

  return ${exit_status}
}

CONFIG_ROOT="${OPENSHIFT_CONFIG_ROOT:-}"
case "${CONFIG_ROOT}" in
  dev)
    CONFIG_ROOT="${OS_ROOT}"
    ;;
  dind)
    CONFIG_ROOT="/tmp/openshift-dind-cluster/\
${OPENSHIFT_INSTANCE_PREFIX:-openshift}"
    if [[ ! -d "${CONFIG_ROOT}" ]]; then
      os::log::error "OPENSHIFT_CONFIG_ROOT=dind but dind cluster not found"
      os::log::info  "To launch a cluster: hack/dind-cluster.sh start"
      exit 1
    fi
    ;;
  *)
    if [[ -n "${CONFIG_ROOT}" ]]; then
      CONFIG_FILE="${CONFIG_ROOT}/openshift.local.config/master/admin.kubeconfig"
      if [[ ! -f "${CONFIG_FILE}" ]]; then
        os::log::error "${CONFIG_FILE} not found"
        exit 1
      fi
    fi
    ;;
esac

TEST_EXTRA_ARGS="$@"

if [[ "${OPENSHIFT_SKIP_BUILD:-false}" = "true" ]] &&
     [[ -n $(os::build::find-binary extended.test) ]]; then
  os::log::warn "Skipping rebuild of test binary due to OPENSHIFT_SKIP_BUILD=true"
else
  # cgo must be disabled to have the symbol table available
  CGO_ENABLED=0 hack/build-go.sh test/extended/extended.test
fi
TEST_BINARY="${OS_ROOT}/$(os::build::find-binary extended.test)"

# enable-selinux/disable-selinux use the shared control variable
# SELINUX_DISABLED to determine whether to re-enable selinux after it
# has been disabled.  The goal is to allow temporary disablement of
# selinux enforcement while avoiding enabling enforcement in an
# environment where it is not already enabled.
SELINUX_DISABLED=0

function enable-selinux() {
  if [ "${SELINUX_DISABLED}" = "1" ]; then
    os::log::info "Re-enabling selinux enforcement"
    sudo setenforce 1
    SELINUX_DISABLED=0
  fi
}

function disable-selinux() {
  if selinuxenabled && [ "$(getenforce)" = "Enforcing" ]; then
    os::log::info "Temporarily disabling selinux enforcement"
    sudo setenforce 0
    SELINUX_DISABLED=1
  fi
}

os::log::info "Starting 'networking' extended tests"
if [[ -n "${CONFIG_ROOT}" ]]; then
  KUBECONFIG="$(get-kubeconfig-from-root "${CONFIG_ROOT}")"
  os::log::info "KUBECONFIG=${KUBECONFIG}"
  run-extended-tests "${KUBECONFIG}"
elif [[ -n "${OPENSHIFT_TEST_KUBECONFIG:-}" ]]; then
  os::log::info "KUBECONFIG=${OPENSHIFT_TEST_KUBECONFIG}"
  # Run tests against an existing cluster
  run-extended-tests "${OPENSHIFT_TEST_KUBECONFIG}"
else
  # For each plugin, run tests against a test-managed cluster

  # Use a unique instance prefix to ensure the names of the test dind
  # containers will not clash with the names of non-test containers.
  export OPENSHIFT_CLUSTER_ID="nettest"
  # TODO(marun) Discover these names instead of hard-coding
  CONTAINER_NAMES=(
    "${OPENSHIFT_CLUSTER_ID}-master"
    "${OPENSHIFT_CLUSTER_ID}-node-1"
    "${OPENSHIFT_CLUSTER_ID}-node-2"
  )

  os::util::environment::setup_tmpdir_vars "test-extended/networking"

  os::log::system::start

  os::log::info "Building docker-in-docker images"
  ${CLUSTER_CMD} build-images

  # Ensure cleanup on error
  ENABLE_SELINUX=0
  function cleanup-dind {
    local exit_code=$?
    if [[ "${DIND_CLEANUP_REQUIRED}" = "1" ]]; then
      os::log::info "Shutting down docker-in-docker cluster"
      ${CLUSTER_CMD} stop || true
    fi
    enable-selinux || true
    if [[ "${TEST_FAILURES}" = "0" ]]; then
      os::log::info "No test failures were detected"
    else
      os::log::error "${TEST_FAILURES} plugin(s) failed one or more tests"
    fi
    # Return non-zero for either command or test failures
    if [[ "${exit_code}" = "0" ]]; then
      exit_code="${TEST_FAILURES}"
    else
      os::log::error "Exiting with code ${exit_code}"
    fi
    exit $exit_code
  }
  trap "exit" INT TERM
  trap "cleanup-dind" EXIT

  # Docker-in-docker is not compatible with selinux
  disable-selinux

  # Skip the subnet tests during a minimal test run
  if [[ -z "${NETWORKING_E2E_MINIMAL}" ]]; then
    # Ignore deployment errors for a given plugin to allow other plugins
    # to be tested.
    test-osdn-plugin "subnet" "redhat/openshift-ovs-subnet" "false" || true
  fi

  test-osdn-plugin "multitenant" "redhat/openshift-ovs-multitenant" "true" || true
fi
