#!/bin/bash

# This command checks that the built commands can function together for
# simple scenarios.  It does not require Docker so it can run in travis.

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

function cleanup()
{
    out=$?
    pkill -P $$
    set +e
    kill_all_processes

    if [ $out -ne 0 ]; then
        echo "[FAIL] !!!!! Test Failed !!!!"
        echo
        cat "${LOG_DIR}/openshift.log"
        echo
        echo -------------------------------------
        echo
    else
        if path=$(go tool -n pprof 2>&1); then
          echo
          echo "pprof: top output"
          echo
          go tool pprof -text ./_output/local/bin/$(os::util::host_platform)/openshift cpu.pprof
        fi

        echo
        echo "Complete"
    fi

    ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"
    exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

set -e

function find_tests {
  cd "${OS_ROOT}"
  find "${1}" -name '*.sh' | sort -u
}
tests=( $(find_tests ${1:-test/cmd}) )

# Setup environment

# test-cmd specific defaults
BASETMPDIR=${USE_TEMP:-$(mkdir -p /tmp/openshift-cmd && mktemp -d /tmp/openshift-cmd/XXXX)}
API_HOST=${API_HOST:-127.0.0.1}
export API_PORT=${API_PORT:-28443}

setup_env_vars
mkdir -p "${ETCD_DATA_DIR}" "${VOLUME_DIR}" "${FAKE_HOME_DIR}" "${MASTER_CONFIG_DIR}" "${NODE_CONFIG_DIR}" "${LOG_DIR}"

ETCD_HOST=${ETCD_HOST:-127.0.0.1}
ETCD_PORT=${ETCD_PORT:-24001}
ETCD_PEER_PORT=${ETCD_PEER_PORT:-27001}
MASTER_ADDR="${API_SCHEME}://${API_HOST}:${API_PORT}"
PUBLIC_MASTER_HOST="${PUBLIC_MASTER_HOST:-${API_HOST}}"

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

# validate config that was generated
[ "$(openshift ex validate master-config ${MASTER_CONFIG_DIR}/master-config.yaml 2>&1 | grep SUCCESS)" ]
[ "$(openshift ex validate node-config ${NODE_CONFIG_DIR}/node-config.yaml 2>&1 | grep SUCCESS)" ]
# breaking the config fails the validation check
cp ${MASTER_CONFIG_DIR}/master-config.yaml ${BASETMPDIR}/master-config-broken.yaml
sed -i '5,10d' ${BASETMPDIR}/master-config-broken.yaml
[ "$(openshift ex validate master-config ${BASETMPDIR}/master-config-broken.yaml 2>&1 | grep error)" ]

cp ${NODE_CONFIG_DIR}/node-config.yaml ${BASETMPDIR}/node-config-broken.yaml
sed -i '5,10d' ${BASETMPDIR}/node-config-broken.yaml
[ "$(openshift ex validate node-config ${BASETMPDIR}/node-config-broken.yaml 2>&1 | grep ERROR)" ]
echo "validation: ok"

# Don't try this at home.  We don't have flags for setting etcd ports in the config, but we want deconflicted ones.  Use sed to replace defaults in a completely unsafe way
os::util::sed "s/:4001$/:${ETCD_PORT}/g" ${SERVER_CONFIG_DIR}/master/master-config.yaml
os::util::sed "s/:7001$/:${ETCD_PEER_PORT}/g" ${SERVER_CONFIG_DIR}/master/master-config.yaml

# Start openshift
OPENSHIFT_ON_PANIC=crash openshift start master \
  --config=${MASTER_CONFIG_DIR}/master-config.yaml \
  --loglevel=4 \
  1>&2 2>"${LOG_DIR}/openshift.log" &
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

# ensure that DisabledFeatures aren't written to config files
! grep -i '\<disabledFeatures\>' \
	"${MASTER_CONFIG_DIR}/master-config.yaml" \
	"${BASETMPDIR}/atomic.local.config/master/master-config.yaml" \
	"${NODE_CONFIG_DIR}/node-config.yaml"

# test client not configured
[ "$(oc get services 2>&1 | grep 'Error in configuration')" ]

# Set KUBERNETES_MASTER for oc from now on
export KUBERNETES_MASTER="${API_SCHEME}://${API_HOST}:${API_PORT}"

# Set certificates for oc from now on
if [[ "${API_SCHEME}" == "https" ]]; then
    # test bad certificate
    [ "$(oc get services 2>&1 | grep 'certificate signed by unknown authority')" ]
fi


# login and logout tests
# --token and --username are mutually exclusive
[ "$(oc login ${KUBERNETES_MASTER} -u test-user --token=tmp --insecure-skip-tls-verify 2>&1 | grep 'mutually exclusive')" ]
# must only accept one arg (server)
[ "$(oc login https://server1 https://server2.com 2>&1 | grep 'Only the server URL may be specified')" ]
# logs in with a valid certificate authority
oc login ${KUBERNETES_MASTER} --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" -u test-user -p anything --api-version=v1beta3
grep -q "v1beta3" ${HOME}/.kube/config
oc logout
# logs in skipping certificate check
oc login ${KUBERNETES_MASTER} --insecure-skip-tls-verify -u test-user -p anything
# logs in by an existing and valid token
temp_token=$(oc config view -o template --template='{{range .users}}{{ index .user.token }}{{end}}')
[ "$(oc login --token=${temp_token} 2>&1 | grep 'using the token provided')" ]
oc logout
# properly parse server port
[ "$(oc login https://server1:844333 2>&1 | grep 'Not a valid port')" ]
# properly handle trailing slash
oc login --server=${KUBERNETES_MASTER} --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" -u test-user -p anything
# create a new project
oc new-project project-foo --display-name="my project" --description="boring project description"
[ "$(oc project | grep 'Using project "project-foo"')" ]
# new user should get default context
oc login --server=${KUBERNETES_MASTER} --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" -u new-and-unknown-user -p anything
[ "$(oc config view | grep current-context | grep /${API_HOST}:${API_PORT}/new-and-unknown-user)" ]
# denies access after logging out
oc logout
[ -z "$(oc get pods | grep 'system:anonymous')" ]

# log in as an image-pruner and test that oadm prune images works against the atomic binary
oadm policy add-cluster-role-to-user system:image-pruner pruner --config="${MASTER_CONFIG_DIR}/admin.kubeconfig"
oc login --server=${KUBERNETES_MASTER} --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" -u pruner -p anything
# this shouldn't fail but instead output "Dry run enabled - no modifications will be made. Add --confirm to remove images"
oadm prune images

# log in and set project to use from now on
oc login --server=${KUBERNETES_MASTER} --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" -u test-user -p anything
oc get projects
oc project project-foo
[ "$(oc config view | grep current-context | grep project-foo/${API_HOST}:${API_PORT}/test-user)" ]
[ "$(oc whoami | grep 'test-user')" ]
[ -n "$(oc whoami -t)" ]
[ -n "$(oc whoami -c)" ]

# test config files from the --config flag
oc get services --config="${MASTER_CONFIG_DIR}/admin.kubeconfig"

# test config files from env vars
KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig" oc get services

# test config files in the home directory
mkdir -p ${HOME}/.kube
cp ${MASTER_CONFIG_DIR}/admin.kubeconfig ${HOME}/.kube/config
oc get services
mv ${HOME}/.kube/config ${HOME}/.kube/non-default-config
echo "config files: ok"

# from this point every command will use config from the KUBECONFIG env var
export KUBECONFIG="${HOME}/.kube/non-default-config"
export CLUSTER_ADMIN_CONTEXT=$(oc config view --flatten -o template --template='{{index . "current-context"}}')


# NOTE: Do not add tests here, add them to test/cmd/*.
# Tests should assume they run in an empty project, and should be reentrant if possible
# to make it easy to run individual tests
for test in "${tests[@]}"; do
  echo
  echo "++ ${test}"
  name=$(basename ${test} .sh)

  # switch back to a standard identity. This prevents individual tests from changing contexts and messing up other tests
  oc project ${CLUSTER_ADMIN_CONTEXT}
  oc new-project "cmd-${name}"
  ${test}
  oc delete project "cmd-${name}"
done


# Done
echo
echo
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/metrics" "metrics: " 0.25 80
echo
echo
echo "test-cmd: ok"
