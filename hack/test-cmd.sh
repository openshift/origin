#!/bin/bash

# This command checks that the built commands can function together for
# simple scenarios.  It does not require Docker so it can run in travis.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

os::log::install_errexit

function cleanup()
{
    out=$?
    pkill -P $$

    if [ $out -ne 0 ]; then
        echo "[FAIL] !!!!! Test Failed !!!!"
        echo
        cat "${TEMP_DIR}/openshift.log"
        echo
        echo -------------------------------------
        echo
    else
        if path=$(go tool -n pprof 2>&1); then
          echo
          echo "pprof: top output"
          echo
          set +e
          go tool pprof -text ./_output/local/go/bin/openshift cpu.pprof
        fi

        echo
        echo "Complete"
    fi
    exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

set -e

function find_tests {
  cd "${OS_ROOT}"
  find "${1}" -name '*.sh' -print0 | sort -u | xargs -0 -n1 printf "%s\n"
}
tests=( $(find_tests ${1:-test/cmd}) )

# Prevent user environment from colliding with the test setup
unset KUBECONFIG

# Use either the latest release built images, or latest.
if [[ -z "${USE_IMAGES-}" ]]; then
  tag="latest"
  if [[ -e "${OS_ROOT}/_output/local/releases/.commit" ]]; then
    COMMIT="$(cat "${OS_ROOT}/_output/local/releases/.commit")"
    tag="${COMMIT}"
  fi
  USE_IMAGES="openshift/origin-\${component}:${tag}"
fi
export USE_IMAGES

ETCD_HOST=${ETCD_HOST:-127.0.0.1}
ETCD_PORT=${ETCD_PORT:-4001}
API_SCHEME=${API_SCHEME:-https}
API_PORT=${API_PORT:-8443}
API_HOST=${API_HOST:-127.0.0.1}
MASTER_ADDR="${API_SCHEME}://${API_HOST}:${API_PORT}"
PUBLIC_MASTER_HOST="${PUBLIC_MASTER_HOST:-${API_HOST}}"
KUBELET_SCHEME=${KUBELET_SCHEME:-https}
KUBELET_HOST=${KUBELET_HOST:-127.0.0.1}
KUBELET_PORT=${KUBELET_PORT:-10250}

TEMP_DIR=${USE_TEMP:-$(mkdir -p /tmp/openshift-cmd && mktemp -d /tmp/openshift-cmd/XXXX)}
ETCD_DATA_DIR="${TEMP_DIR}/etcd"
VOLUME_DIR="${TEMP_DIR}/volumes"
FAKE_HOME_DIR="${TEMP_DIR}/openshift.local.home"
SERVER_CONFIG_DIR="${TEMP_DIR}/openshift.local.config"
MASTER_CONFIG_DIR="${SERVER_CONFIG_DIR}/master"
NODE_CONFIG_DIR="${SERVER_CONFIG_DIR}/node-${KUBELET_HOST}"
CONFIG_DIR="${TEMP_DIR}/configs"
mkdir -p "${ETCD_DATA_DIR}" "${VOLUME_DIR}" "${FAKE_HOME_DIR}" "${MASTER_CONFIG_DIR}" "${NODE_CONFIG_DIR}" "${CONFIG_DIR}"

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

# set path so OpenShift is available
GO_OUT="${OS_ROOT}/_output/local/go/bin"
export PATH="${GO_OUT}:${PATH}"

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


# Start openshift
OPENSHIFT_ON_PANIC=crash openshift start \
  --master-config=${MASTER_CONFIG_DIR}/master-config.yaml \
  --node-config=${NODE_CONFIG_DIR}/node-config.yaml \
  --loglevel=4 \
  1>&2 2>"${TEMP_DIR}/openshift.log" &
OS_PID=$!

if [[ "${API_SCHEME}" == "https" ]]; then
    export CURL_CA_BUNDLE="${MASTER_CONFIG_DIR}/ca.crt"
    export CURL_CERT="${MASTER_CONFIG_DIR}/admin.crt"
    export CURL_KEY="${MASTER_CONFIG_DIR}/admin.key"
fi

# set the home directory so we don't pick up the users .config
export HOME="${FAKE_HOME_DIR}"

wait_for_url "${KUBELET_SCHEME}://${KUBELET_HOST}:${KUBELET_PORT}/healthz" "kubelet: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1beta3/nodes/${KUBELET_HOST}" "apiserver(nodes): " 0.25 80

# profile the cli commands
export OPENSHIFT_PROFILE="${CLI_PROFILE-}"

#
# Begin tests
#

# ensure that DisabledFeatures aren't written to config files
! grep -i '\<disabledFeatures\>' \
	"${MASTER_CONFIG_DIR}/master-config.yaml" \
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
oc login --server=${KUBERNETES_MASTER}/ --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" -u test-user -p anything
# create a new project
oc new-project project-foo --display-name="my project" --description="boring project description"
[ "$(oc project | grep 'Using project "project-foo"')" ]
# denies access after logging out
oc logout
[ -z "$(oc get pods | grep 'system:anonymous')" ]

# log in and set project to use from now on
oc login --server=${KUBERNETES_MASTER} --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" -u test-user -p anything
oc get projects
oc project project-foo
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

# Test access to /console/
$(which curl) -sfL --max-time 5 "${API_SCHEME}://${API_HOST}:${API_PORT}/console/" | grep '<title>'
echo "console: ok"

# from this point every command will use config from the KUBECONFIG env var
export KUBECONFIG="${HOME}/.kube/non-default-config"


# NOTE: Do not add tests here, add them to test/cmd/*.
# Tests should assume they run in an empty project, and should be reentrant if possible
# to make it easy to run individual tests
for test in "${tests[@]}"; do
  echo
  echo "++ ${test}"
  name=$(basename ${test} .sh)
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
echo "openshift: ok"

# Atomic Enterprise tests *****************************************************
# create master config as atomic-enterprise just to test it works
kill -TERM $OS_PID
rm -rf ${SERVER_CONFIG_DIR} ${HOME}/.kube
unset KUBECONFIG

atomic-enterprise admin ca create-master-certs \
  --overwrite=false \
  --cert-dir="${MASTER_CONFIG_DIR}" \
  --hostnames="${SERVER_HOSTNAME_LIST}" \
  --master="${MASTER_ADDR}" \
  --public-master="${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}"

atomic-enterprise admin create-node-config \
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

atomic-enterprise start \
  --write-config=${SERVER_CONFIG_DIR} \
  --create-certs=false \
  --master="${API_SCHEME}://${API_HOST}:${API_PORT}" \
  --listen="${API_SCHEME}://${API_HOST}:${API_PORT}" \
  --hostname="${KUBELET_HOST}" \
  --volume-dir="${VOLUME_DIR}" \
  --etcd-dir="${ETCD_DATA_DIR}" \
  --images="${USE_IMAGES}"

# ensure that DisabledFeatures aren't written to config files
! grep -i '\<disabledFeatures\>' \
	"${MASTER_CONFIG_DIR}/master-config.yaml" \
	"${NODE_CONFIG_DIR}/node-config.yaml"

# Start atomic-enterprise
OPENSHIFT_ON_PANIC=crash atomic-enterprise start \
  --master-config=${MASTER_CONFIG_DIR}/master-config.yaml \
  --node-config=${NODE_CONFIG_DIR}/node-config.yaml \
  --loglevel=4 \
  1>&2 2>"${TEMP_DIR}/atomic-enterprise.log" &
OS_PID=$!

# Wait for atomic-enterprise to become available
wait_for_url "${KUBELET_SCHEME}://${KUBELET_HOST}:${KUBELET_PORT}/healthz" "kubelet: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 80
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1beta3/nodes/${KUBELET_HOST}" "apiserver(nodes): " 0.25 80

# Test log(in|out) with atomic-enterprise
# --token and --username are mutually exclusive
[ "$(atomic-enterprise cli login -u test-user --token=tmp --insecure-skip-tls-verify 2>&1 | grep 'mutually exclusive')" ]
# must only accept one arg (server)
[ "$(atomic-enterprise cli login https://server1 https://server2.com 2>&1 | grep 'Only the server URL may be specified')" ]
# logs in with a valid certificate authority
atomic-enterprise cli login ${KUBERNETES_MASTER} --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" -u test-user -p anything --api-version=v1beta3
grep -q "v1beta3" ${HOME}/.kube/config
atomic-enterprise cli logout
# logs in skipping certificate check
atomic-enterprise cli login ${KUBERNETES_MASTER} --insecure-skip-tls-verify -u test-user -p anything
# logs in by an existing and valid token
temp_token=$(atomic-enterprise cli config view -o template --template='{{range .users}}{{ index .user.token }}{{end}}')
[ "$(atomic-enterprise cli login --token=${temp_token} 2>&1 | grep 'using the token provided')" ]
atomic-enterprise cli logout
# properly parse server port
[ "$(atomic-enterprise cli login https://server1:844333 2>&1 | grep 'Not a valid port')" ]
# properly handle trailing slash
atomic-enterprise cli login --server=${KUBERNETES_MASTER} --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" -u test-user -p anything
# create a new project
atomic-enterprise cli new-project project-bar --display-name="my project" --description="boring project description"
[ "$(atomic-enterprise cli project | grep 'Using project "project-bar"')" ]
# denies access after logging out
atomic-enterprise cli logout
[ -z "$(atomic-enterprise cli get pods | grep 'system:anonymous')" ]

# Test access to /console is forbidden
$(which curl) --max-time 5 "${API_SCHEME}://${API_HOST}:${API_PORT}/console/" | grep -qi '\("code"\s*:\s*403"\|"reason"\s*:\s*"Forbidden"\)'
echo "console-disabled: ok"

echo
echo "test-cmd: ok"
