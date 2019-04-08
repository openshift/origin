#!/usr/bin/env bash

# This command checks that the built commands can function together for
# simple scenarios.  It does not require Docker so it can run in travis.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
os::util::environment::setup_time_vars
source "${OS_ROOT}/hack/local-up-master/lib.sh"

function cleanup() {
  return_code=$?
  os::test::junit::generate_report
  os::cleanup::all
  os::util::describe_return_code "${return_code}"
  clusterup::cleanup
  clusterup::cleanup_config
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

# indicate we are running an integration test
export OS_INTEGRATION_TEST='true'

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

export ALLOWED_REGISTRIES='[{"domainName":"172.30.30.30:5000"},{"domainName":"myregistry.com"},{"domainName":"registry.centos.org"},{"domainName":"docker.io"},{"domainName":"gcr.io"},{"domainName":"quay.io"},{"domainName":"*.redhat.com"},{"domainName":"*.docker.io"},{"domainName":"registry.redhat.io"}]'

export PATH="$( ./hack/install-etcd.sh --export-path )":$PATH
LOCALUP_ROOT=$(mktemp -d) localup::init_master

export ADMIN_KUBECONFIG="${LOCALUP_CONFIG}/admin.kubeconfig"

os::test::junit::declare_suite_start "cmd/version"
os::cmd::expect_success_and_not_text "KUBECONFIG='${LOCALUP_CONFIG}/admin.kubeconfig' oc version" "did you specify the right host or port"
os::cmd::expect_success_and_not_text "KUBECONFIG='' oc version" "Missing or incomplete configuration info"
os::test::junit::declare_suite_end

export HOME="${FAKE_HOME_DIR}"
${USE_SUDO:+sudo} rm -rf "${FAKE_HOME_DIR}"
mkdir -p "${HOME}/.kube"
cp "${LOCALUP_CONFIG}/admin.kubeconfig" "${HOME}/.kube/non-default-config"
export KUBECONFIG="${HOME}/.kube/non-default-config"
export KUBERNETES_MASTER="${API_SCHEME}://${API_HOST}:${API_PORT}"

# Store starting cluster RBAC to allow it to be restored during tests that mutate the global policy
BASE_RBAC_DATA="$( mktemp "${BASETMPDIR}/base_rbac_data_XXXXX" )"
export BASE_RBAC_DATA

oc get clusterroles.rbac,clusterrolebindings.rbac -o yaml | grep -v resourceVersion > "${BASE_RBAC_DATA}"
# these should probably be set up in localup::init_master
export MASTER_CONFIG_DIR="${LOCALUP_CONFIG}/kube-apiserver"
export ETCD_CONFIG_DIR="${LOCALUP_CONFIG}/etcd"
export ETCD_CLIENT_CERT="${ETCD_CONFIG_DIR}/client-etcd-client.crt"
export ETCD_CLIENT_KEY="${ETCD_CONFIG_DIR}/client-etcd-client.key"
export ETCD_CA_BUNDLE="${ETCD_CONFIG_DIR}/client-ca.crt"
CLUSTER_ADMIN_CONTEXT=$(oc config view --config="${ADMIN_KUBECONFIG}" --flatten -o template --template='{{index . "current-context"}}'); export CLUSTER_ADMIN_CONTEXT
${USE_SUDO:+sudo} chmod -R a+rwX "${ADMIN_KUBECONFIG}"

# wait for apiserver to be ready
os::test::junit::declare_suite_start "setup/localup"
os::cmd::try_until_text "oc get --raw /healthz --as system:unauthenticated --config='${ADMIN_KUBECONFIG}'" 'ok' $(( 160 * second )) 0.25
os::cmd::try_until_success "oc get service kubernetes --namespace default --config='${ADMIN_KUBECONFIG}'" $(( 160 * second )) 0.25
os::cmd::try_until_success "oc login --server=${KUBERNETES_MASTER} --certificate-authority ${MASTER_CONFIG_DIR}/server-ca.crt -u test-user -p anything" $(( 160 * second )) 0.25
# wait for the CRD to be available
os::cmd::try_until_success "oc get clusterresourcequotas --config='${ADMIN_KUBECONFIG}'" $(( 160 * second )) 0.25
os::test::junit::declare_suite_end
os::log::debug "localup server health checks done at: $( date )"

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
  os::cmd::expect_success "oc login --server=${KUBERNETES_MASTER} --certificate-authority ${MASTER_CONFIG_DIR}/server-ca.crt -u test-user -p anything"
  os::cmd::expect_success "oc project ${CLUSTER_ADMIN_CONTEXT}"
  os::cmd::expect_success "oc new-project '${namespace}'"
  # wait for the project cache to catch up and correctly list us in the new project
  os::cmd::try_until_text "oc get projects -o name" "project.project.openshift.io/${namespace}"
  os::test::junit::declare_suite_end

  if ! ${test}; then
    failed="true"
    tail -40 "${LOG_DIR}/openshift-apiserver.log"
  fi

  os::test::junit::declare_suite_start "cmd/${namespace}-namespace-teardown"
  os::cmd::expect_success "oc project '${CLUSTER_ADMIN_CONTEXT}'"
  os::cmd::expect_success "oc delete project '${namespace}'"
  cp ${KUBECONFIG}{.bak,}  # since nothing ever gets deleted from kubeconfig, reset it
  os::test::junit::declare_suite_end
done

os::log::debug "Metrics information logged to ${LOG_DIR}/metrics.log"
oc get --raw /metrics --kubeconfig="${MASTER_CONFIG_DIR}/admin.kubeconfig"> "${LOG_DIR}/metrics.log"

if [[ -n "${failed:-}" ]]; then
    exit 1
fi
echo "test-cmd: ok"
