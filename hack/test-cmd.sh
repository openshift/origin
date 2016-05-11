#!/bin/bash

# This command checks that the built commands can function together for
# simple scenarios.  It does not require Docker so it can run in travis.

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
cd "${OS_ROOT}"
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/lib/log.sh"
source "${OS_ROOT}/hack/lib/util/environment.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
source "${OS_ROOT}/hack/lib/test/junit.sh"
os::log::install_errexit
os::util::environment::setup_time_vars

function cleanup()
{
    out=$?
    pkill -P $$
    set +e
    kill_all_processes

    echo "[INFO] Dumping etcd contents to ${ARTIFACT_DIR}/etcd_dump.json"
    set_curl_args 0 1
    curl -s ${clientcert_args} -L "${API_SCHEME}://${API_HOST}:${ETCD_PORT}/v2/keys/?recursive=true" > "${ARTIFACT_DIR}/etcd_dump.json"
    echo

    # we keep a JSON dump of etcd data so we do not need to keep the binary store
    local sudo="${USE_SUDO:+sudo}"
    ${sudo} rm -rf "${ETCD_DATA_DIR}"

    if [ $out -ne 0 ]; then
        echo "[FAIL] !!!!! Test Failed !!!!"
        echo
        tail -40 "${LOG_DIR}/openshift.log"
        echo
        echo -------------------------------------
        echo
    else
        if path=$(go tool -n pprof 2>&1); then
          echo
          echo "pprof: top output"
          echo
          go tool pprof -text ./_output/local/bin/$(os::util::host_platform)/openshift cpu.pprof | head -120
        fi

        echo
        echo "Complete"
    fi

    # TODO(skuznets): un-hack this nonsense once traps are in a better state
    if [[ -n "${JUNIT_REPORT_OUTPUT:-}" ]]; then
      # get the jUnit output file into a workable state in case we crashed in the middle of testing something
      os::test::junit::reconcile_output

      # check that we didn't mangle jUnit output
      os::test::junit::check_test_counters

      # use the junitreport tool to generate us a report
      "${OS_ROOT}/hack/build-go.sh" tools/junitreport
      junitreport="$(os::build::find-binary junitreport)"

      if [[ -z "${junitreport}" ]]; then
          echo "It looks as if you don't have a compiled junitreport binary"
          echo
          echo "If you are running from a clone of the git repo, please run"
          echo "'./hack/build-go.sh tools/junitreport'."
          exit 1
      fi

      cat "${JUNIT_REPORT_OUTPUT}"                             \
        | "${junitreport}" --type oscmd                        \
                           --suites nested                     \
                           --roots github.com/openshift/origin \
                           --output "${ARTIFACT_DIR}/report.xml"
      cat "${ARTIFACT_DIR}/report.xml" | "${junitreport}" summarize
    fi 

    ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"
    exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

set -e

function find_tests {
  find "${OS_ROOT}/test/cmd" -name '*.sh' | grep -E "${1}" | sort -u
}
tests=( $(find_tests ${1:-.*}) )

# Setup environment

# test-cmd specific defaults
API_HOST=${API_HOST:-127.0.0.1}
export API_PORT=${API_PORT:-28443}

export ETCD_HOST=${ETCD_HOST:-127.0.0.1}
export ETCD_PORT=${ETCD_PORT:-24001}
export ETCD_PEER_PORT=${ETCD_PEER_PORT:-27001}

os::util::environment::setup_all_server_vars "test-cmd/"
reset_tmp_dir

# Allow setting $JUNIT_REPORT to toggle output behavior
if [[ -n "${JUNIT_REPORT:-}" ]]; then
  export JUNIT_REPORT_OUTPUT="${LOG_DIR}/raw_test_output.log"
fi

echo "Logging to ${LOG_DIR}..."

os::log::start_system_logger

# Prevent user environment from colliding with the test setup
unset KUBECONFIG

# test wrapper functions
${OS_ROOT}/hack/test-util.sh > ${LOG_DIR}/wrappers_test.log 2>&1

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

# Check openshift version
out=$(openshift version)
echo openshift: $out

# profile the web
export OPENSHIFT_PROFILE="${WEB_PROFILE-}"

# Specify the scheme and port for the listen address, but let the IP auto-discover. Set --public-master to localhost, for a stable link to the console.
echo "[INFO] Create certificates for the OpenShift server to ${MASTER_CONFIG_DIR}"
# find the same IP that openshift start will bind to.  This allows access from pods that have to talk back to master
SERVER_HOSTNAME_LIST="${PUBLIC_MASTER_HOST},$(openshift start --print-ip),localhost"

openshift admin ca create-master-certs \
  --overwrite=false \
  --cert-dir="${MASTER_CONFIG_DIR}" \
  --hostnames="${SERVER_HOSTNAME_LIST}" \
  --master="${MASTER_ADDR}" \
  --public-master="${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}"

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

# create openshift config
openshift start \
  --write-config=${SERVER_CONFIG_DIR} \
  --create-certs=false \
  --master="${API_SCHEME}://${API_HOST}:${API_PORT}" \
  --listen="${API_SCHEME}://${API_HOST}:${API_PORT}" \
  --hostname="${KUBELET_HOST}" \
  --volume-dir="${VOLUME_DIR}" \
  --etcd-dir="${ETCD_DATA_DIR}" \
  --images="${USE_IMAGES}"

# Don't try this at home.  We don't have flags for setting etcd ports in the config, but we want deconflicted ones.  Use sed to replace defaults in a completely unsafe way
os::util::sed "s/:4001$/:${ETCD_PORT}/g" ${SERVER_CONFIG_DIR}/master/master-config.yaml
os::util::sed "s/:7001$/:${ETCD_PEER_PORT}/g" ${SERVER_CONFIG_DIR}/master/master-config.yaml

# Start openshift
OPENSHIFT_ON_PANIC=crash openshift start master \
  --config=${MASTER_CONFIG_DIR}/master-config.yaml \
  --loglevel=5 \
  &>"${LOG_DIR}/openshift.log" &
OS_PID=$!

if [[ "${API_SCHEME}" == "https" ]]; then
    export CURL_CA_BUNDLE="${MASTER_CONFIG_DIR}/ca.crt"
    export CURL_CERT="${MASTER_CONFIG_DIR}/admin.crt"
    export CURL_KEY="${MASTER_CONFIG_DIR}/admin.key"
fi

wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 80

# profile the cli commands
export OPENSHIFT_PROFILE="${CLI_PROFILE-}"

#
# Begin tests
#

# create master config as atomic-enterprise just to test it works
atomic-enterprise start \
  --write-config="${BASETMPDIR}/atomic.local.config" \
  --create-certs=true \
  --master="${API_SCHEME}://${API_HOST}:${API_PORT}" \
  --listen="${API_SCHEME}://${API_HOST}:${API_PORT}" \
  --hostname="${KUBELET_HOST}" \
  --volume-dir="${VOLUME_DIR}" \
  --etcd-dir="${ETCD_DATA_DIR}" \
  --images="${USE_IMAGES}"

os::test::junit::declare_suite_start "cmd/validatation"
# validate config that was generated
os::cmd::expect_success_and_text "openshift ex validate master-config ${MASTER_CONFIG_DIR}/master-config.yaml" 'SUCCESS'
os::cmd::expect_success_and_text "openshift ex validate node-config ${NODE_CONFIG_DIR}/node-config.yaml" 'SUCCESS'
# breaking the config fails the validation check
cp ${MASTER_CONFIG_DIR}/master-config.yaml ${BASETMPDIR}/master-config-broken.yaml
os::util::sed '7,12d' ${BASETMPDIR}/master-config-broken.yaml
os::cmd::expect_failure_and_text "openshift ex validate master-config ${BASETMPDIR}/master-config-broken.yaml" 'ERROR'

cp ${NODE_CONFIG_DIR}/node-config.yaml ${BASETMPDIR}/node-config-broken.yaml
os::util::sed '5,10d' ${BASETMPDIR}/node-config-broken.yaml
os::cmd::expect_failure_and_text "openshift ex validate node-config ${BASETMPDIR}/node-config-broken.yaml" 'ERROR'
echo "validation: ok"
os::test::junit::declare_suite_end

os::test::junit::declare_suite_start "cmd/config"
# ensure that DisabledFeatures aren't written to config files
os::cmd::expect_success_and_text "cat ${MASTER_CONFIG_DIR}/master-config.yaml" 'disabledFeatures: null'
os::cmd::expect_success_and_text "cat ${BASETMPDIR}/atomic.local.config/master/master-config.yaml" 'disabledFeatures: null'

# test client not configured
os::cmd::expect_failure_and_text "oc get services" 'No configuration file found, please login'
unused_port="33333"
# setting env bypasses the not configured message
os::cmd::expect_failure_and_text "KUBERNETES_MASTER=http://${API_HOST}:${unused_port} oc get services" 'did you specify the right host or port'
# setting --server bypasses the not configured message
os::cmd::expect_failure_and_text "oc get services --server=http://${API_HOST}:${unused_port}" 'did you specify the right host or port'

# Set KUBERNETES_MASTER for oc from now on
export KUBERNETES_MASTER="${API_SCHEME}://${API_HOST}:${API_PORT}"

# Set certificates for oc from now on
if [[ "${API_SCHEME}" == "https" ]]; then
    # test bad certificate
    os::cmd::expect_failure_and_text "oc get services" 'certificate signed by unknown authority'
fi

# login and logout tests
# bad token should error
os::cmd::expect_failure_and_text "oc login ${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' --token=badvalue" 'The token provided is invalid or expired'
# bad --api-version should error
os::cmd::expect_failure_and_text "oc login ${KUBERNETES_MASTER} -u test-user -p test-password --api-version=foo/bar/baz" 'error.*foo/bar/baz'
# --token and --username are mutually exclusive
os::cmd::expect_failure_and_text "oc login ${KUBERNETES_MASTER} -u test-user --token=tmp --insecure-skip-tls-verify" 'mutually exclusive'
# must only accept one arg (server)
os::cmd::expect_failure_and_text "oc login https://server1 https://server2.com" 'Only the server URL may be specified'
# logs in with a valid certificate authority
os::cmd::expect_success "oc login ${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u test-user -p anything --api-version=v1"
os::cmd::expect_success_and_text "cat ${HOME}/.kube/config" "v1"
os::cmd::expect_success 'oc logout'
# logs in skipping certificate check
os::cmd::expect_success "oc login ${KUBERNETES_MASTER} --insecure-skip-tls-verify -u test-user -p anything"
# logs in by an existing and valid token
temp_token=$(oc config view -o template --template='{{range .users}}{{ index .user.token }}{{end}}')
os::cmd::expect_success_and_text "oc login --token=${temp_token}" 'using the token provided'
os::cmd::expect_success 'oc logout'
# properly parse server port
os::cmd::expect_failure_and_text 'oc login https://server1:844333' 'Not a valid port'
# properly handle trailing slash
os::cmd::expect_success "oc login --server=${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u test-user -p anything"
# create a new project
os::cmd::expect_success "oc new-project project-foo --display-name='my project' --description='boring project description'"
os::cmd::expect_success_and_text "oc project" 'Using project "project-foo"'
# new user should get default context
os::cmd::expect_success "oc login --server=${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u new-and-unknown-user -p anything"
os::cmd::expect_success_and_text 'oc config view' "current-context.+/${API_HOST}:${API_PORT}/new-and-unknown-user"
# denies access after logging out
os::cmd::expect_success 'oc logout'
os::cmd::expect_failure_and_text 'oc get pods' '"system:anonymous" cannot list pods'

# log in as an image-pruner and test that oadm prune images works against the atomic binary
os::cmd::expect_success "oadm policy add-cluster-role-to-user system:image-pruner pruner --config='${MASTER_CONFIG_DIR}/admin.kubeconfig'"
os::cmd::expect_success "oc login --server=${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u pruner -p anything"
# this shouldn't fail but instead output "Dry run enabled - no modifications will be made. Add --confirm to remove images"
os::cmd::expect_success 'oadm prune images'

# log in and set project to use from now on
VERBOSE=true os::cmd::expect_success "oc login --server=${KUBERNETES_MASTER} --certificate-authority='${MASTER_CONFIG_DIR}/ca.crt' -u test-user -p anything"
VERBOSE=true os::cmd::expect_success 'oc get projects'
VERBOSE=true os::cmd::expect_success 'oc project project-foo'
os::cmd::expect_success_and_text 'oc config view' "current-context.+project-foo/${API_HOST}:${API_PORT}/test-user"
os::cmd::expect_success_and_text 'oc whoami' 'test-user'
os::cmd::expect_success_and_text "oc whoami --config='${MASTER_CONFIG_DIR}/admin.kubeconfig'" 'system:admin'
os::cmd::expect_success_and_text 'oc whoami -t' '.'
os::cmd::expect_success_and_text 'oc whoami -c' '.'

# test config files from the --config flag
os::cmd::expect_success "oc get services --config='${MASTER_CONFIG_DIR}/admin.kubeconfig'"

# test config files from env vars
os::cmd::expect_success "KUBECONFIG='${MASTER_CONFIG_DIR}/admin.kubeconfig' oc get services"

# test config files in the home directory
mkdir -p ${HOME}/.kube
cp ${MASTER_CONFIG_DIR}/admin.kubeconfig ${HOME}/.kube/config
os::cmd::expect_success 'oc get services'
mv ${HOME}/.kube/config ${HOME}/.kube/non-default-config
echo "config files: ok"
os::test::junit::declare_suite_end

# from this point every command will use config from the KUBECONFIG env var
export NODECONFIG="${NODE_CONFIG_DIR}/node-config.yaml"
export KUBECONFIG="${HOME}/.kube/non-default-config"
export CLUSTER_ADMIN_CONTEXT=$(oc config view --flatten -o template --template='{{index . "current-context"}}')

# NOTE: Do not add tests here, add them to test/cmd/*.
# Tests should assume they run in an empty project, and should be reentrant if possible
# to make it easy to run individual tests
cp ${KUBECONFIG}{,.bak}  # keep so we can reset kubeconfig after each test
for test in "${tests[@]}"; do
  echo
  echo "++ ${test}"
  name=$(basename ${test} .sh)

  # switch back to a standard identity. This prevents individual tests from changing contexts and messing up other tests
  oc project ${CLUSTER_ADMIN_CONTEXT}
  oc new-project "cmd-${name}"
  ${test}
  oc project ${CLUSTER_ADMIN_CONTEXT}
  oc delete project "cmd-${name}"
  cp ${KUBECONFIG}{.bak,}  # since nothing ever gets deleted from kubeconfig, reset it
done

# Done
echo
echo
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/metrics" "metrics: " 0.25 80 > "${LOG_DIR}/metrics.log"
grep "request_count" "${LOG_DIR}/metrics.log"
echo
echo
echo "test-cmd: ok"
