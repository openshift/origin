#!/bin/bash

# This command checks that the built commands can function together for
# simple scenarios.  It does not require Docker so it can run in travis.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
os::util::environment::setup_time_vars

function cleanup() {
  return_code=$?
  os::test::junit::generate_report
  os::cleanup::all
  os::util::describe_return_code "${return_code}"
  exit "${return_code}"
}
trap "cleanup" EXIT

function find_tests() {
    local test_regex="${1}"
    local full_test_list=()
    local selected_tests=()

    full_test_list=( $(find "${OS_ROOT}/test/cmd" -name '*.sh') )
    for test in "${full_test_list[@]}"; do
        if grep -q -E "${test_regex}" <<< "${test}"; then
            selected_tests+=( "${test}" )
        fi
    done

    if [[ "${#selected_tests[@]}" -eq 0 ]]; then
        os::log::fatal "No tests were selected due to invalid regex."
    else
        echo "${selected_tests[@]}"
    fi
}
tests=( $(find_tests ${1:-.*}) )

# deconflict ports so we can run in parallel with other test suites
export API_PORT=${API_PORT:-28443}
export ETCD_PORT=${ETCD_PORT:-24001}
export ETCD_PEER_PORT=${ETCD_PEER_PORT:-27001}

# use a network plugin for network tests
export NETWORK_PLUGIN='redhat/openshift-ovs-multitenant'

os::cleanup::tmpdir
os::util::environment::setup_all_server_vars
os::util::ensure_tmpfs "${ETCD_DATA_DIR}"

echo "Logging to ${LOG_DIR}..."

os::log::system::start

# Prevent user environment from colliding with the test setup
unset KUBECONFIG

# handle profiling defaults
profile="${OPENSHIFT_PROFILE-}"
unset OPENSHIFT_PROFILE
if [[ -n "${profile}" ]]; then
    if [[ "${TEST_PROFILE-}" == "cli" ]]; then
        export CLI_PROFILE="${profile}"
    else
        export WEB_PROFILE="${profile}"
    fi
else
  export WEB_PROFILE=cpu
fi

# profile the web
export OPENSHIFT_PROFILE="${WEB_PROFILE-}"
export ALLOWED_REGISTRIES='[{"domainName":"172.30.30.30:5000"},{"domainName":"myregistry.com"},{"domainName":"registry.centos.org"},{"domainName":"docker.io"},{"domainName":"gcr.io"},{"domainName":"quay.io"},{"domainName":"*.redhat.com"},{"domainName":"*.docker.io"},{"domainName":"registry.redhat.io"}]'

os::start::configure_server

os::test::junit::declare_suite_start "cmd/version"
os::cmd::expect_success_and_not_text "KUBECONFIG='${MASTER_CONFIG_DIR}/admin.kubeconfig' oc version" "did you specify the right host or port"
os::cmd::expect_success_and_not_text "KUBECONFIG='' oc version" "Missing or incomplete configuration info"
os::test::junit::declare_suite_end

os::start::master

# profile the cli commands
export OPENSHIFT_PROFILE="${CLI_PROFILE-}"

os::start::registry

export HOME="${FAKE_HOME_DIR}"
mkdir -p "${HOME}/.kube"
cp "${MASTER_CONFIG_DIR}/admin.kubeconfig" "${HOME}/.kube/non-default-config"
export KUBECONFIG="${HOME}/.kube/non-default-config"
export KUBERNETES_MASTER="${API_SCHEME}://${API_HOST}:${API_PORT}"

# NOTE: Do not add tests here, add them to test/cmd/*.
# Tests should assume they run in an empty project, and should be reentrant if possible
# to make it easy to run individual tests
cp ${KUBECONFIG}{,.bak}  # keep so we can reset kubeconfig after each test
for test in "${tests[@]}"; do
  echo
  echo "++ ${test}"
  name=$(basename ${test} .sh)
  namespace="cmd-${name}"

  os::test::junit::declare_suite_start "cmd/${namespace}-namespace-setup"
  # switch back to a standard identity. This prevents individual tests from changing contexts and messing up other tests
  os::cmd::expect_success "oc login --server='${KUBERNETES_MASTER}' --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u test-user -p anything"
  os::cmd::expect_success "oc project ${CLUSTER_ADMIN_CONTEXT}"
  os::cmd::expect_success "oc new-project '${namespace}'"
  # wait for the project cache to catch up and correctly list us in the new project
  os::cmd::try_until_text "oc get projects -o name" "projects/${namespace}"
  os::test::junit::declare_suite_end

  if ! ${test}; then
    failed="true"
    tail -40 "${LOG_DIR}/openshift.log"
  fi

  os::test::junit::declare_suite_start "cmd/${namespace}-namespace-teardown"
  os::cmd::expect_success "oc project '${CLUSTER_ADMIN_CONTEXT}'"
  os::cmd::expect_success "oc delete project '${namespace}'"
  cp ${KUBECONFIG}{.bak,}  # since nothing ever gets deleted from kubeconfig, reset it
  os::test::junit::declare_suite_end
done

os::log::debug "Metrics information logged to ${LOG_DIR}/metrics.log"
oc get --raw /metrics --config="${MASTER_CONFIG_DIR}/admin.kubeconfig"> "${LOG_DIR}/metrics.log"

if [[ -n "${failed:-}" ]]; then
    exit 1
fi
echo "test-cmd: ok"
