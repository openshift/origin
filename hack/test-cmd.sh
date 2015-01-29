#!/bin/bash

# This command checks that the built commands can function together for
# simple scenarios.  It does not require Docker so it can run in travis.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/util.sh"

function cleanup()
{
    out=$?
    pkill -P $$

    if [ $out -ne 0 ]; then
        echo "[FAIL] !!!!! Test Failed !!!!"
    else
        echo
        echo "Complete"
    fi
    exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

set -e

export OPENSHIFT_ON_PANIC=crash

USE_LOCAL_IMAGES=${USE_LOCAL_IMAGES:-true}

ETCD_HOST=${ETCD_HOST:-127.0.0.1}
ETCD_PORT=${ETCD_PORT:-4001}
API_SCHEME=${API_SCHEME:-https}
API_PORT=${API_PORT:-8443}
API_HOST=${API_HOST:-127.0.0.1}
KUBELET_SCHEME=${KUBELET_SCHEME:-http}
KUBELET_PORT=${KUBELET_PORT:-10250}

ETCD_DATA_DIR=$(mktemp -d /tmp/openshift.local.etcd.XXXX)
VOLUME_DIR=$(mktemp -d /tmp/openshift.local.volumes.XXXX)
CERT_DIR="${CERT_DIR:-$(mktemp -d /tmp/openshift.local.certificates.XXXX)}"

# handle profiling defaults
profile="${OPENSHIFT_PROFILE-}"
unset OPENSHIFT_PROFILE
if [[ -n "${profile}" ]]; then
    if [[ "${TEST_PROFILE-}" == "cli" ]]; then
        export CLI_PROFILE="${profile}"
    else
        export WEB_PROFILE="${profile}"
    fi
fi

# set path so OpenShift is available
GO_OUT="${OS_ROOT}/_output/local/go/bin"
export PATH="${GO_OUT}:${PATH}"

# Check openshift version
out=$(openshift version)
echo openshift: $out

# profile the web
export OPENSHIFT_PROFILE="${WEB_PROFILE-}"

# Start openshift
openshift start --master="${API_SCHEME}://${API_HOST}:${API_PORT}" --listen="${API_SCHEME}://${API_HOST}:${API_PORT}" --hostname="${API_HOST}" --volume-dir="${VOLUME_DIR}" --cert-dir="${CERT_DIR}" --etcd-dir="${ETCD_DATA_DIR}" 1>&2 &
OS_PID=$!

if [[ "${API_SCHEME}" == "https" ]]; then
    export CURL_CA_BUNDLE="${CERT_DIR}/admin/root.crt"
fi
wait_for_url "http://${API_HOST}:${KUBELET_PORT}/healthz" "kubelet: " 0.2 60
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.2 60
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1beta1/minions/127.0.0.1" "apiserver(minions): " 0.2 60

# Set KUBERNETES_MASTER for osc
export KUBERNETES_MASTER="${API_SCHEME}://${API_HOST}:${API_PORT}"
if [[ "${API_SCHEME}" == "https" ]]; then
	# Make osc use ${CERT_DIR}/admin/.kubeconfig, and ignore anything in the running user's $HOME dir
	export HOME="${CERT_DIR}/admin"
	export KUBECONFIG="${CERT_DIR}/admin/.kubeconfig"
fi

# profile the cli commands
export OPENSHIFT_PROFILE="${CLI_PROFILE-}"

#
# Begin tests
#

osc config view --kubeconfig=${KUBECONFIG} | grep "user: admin"
echo "config: ok"

osc get pods --match-server-version
osc create -f examples/hello-openshift/hello-pod.json
osc delete pods hello-openshift
echo "pods: ok"

osc get services
osc create -f test/integration/fixtures/test-service.json
osc delete services frontend
echo "services: ok"

osc get minions
echo "minions: ok"

osc get images
osc create -f test/integration/fixtures/test-image.json
osc delete images test
echo "images: ok"

osc get imageRepositories
osc create -f test/integration/fixtures/test-image-repository.json
[ -z "$(osc get imageRepositories test -t "{{.status.dockerImageRepository}}")" ]
osc apply -f examples/sample-app/docker-registry-config.json
[ -n "$(osc get imageRepositories test -t "{{.status.dockerImageRepository}}")" ]
osc delete -f examples/sample-app/docker-registry-config.json
osc delete imageRepositories test
echo "imageRepositories: ok"

osc create -f test/integration/fixtures/test-image-repository.json
osc create -f test/integration/fixtures/test-mapping.json
osc get images
osc get imageRepositories
osc delete imageRepositories test
echo "imageRepositoryMappings: ok"

osc get routes
osc create -f test/integration/fixtures/test-route.json create routes
osc delete routes testroute
echo "routes: ok"

osc get deploymentConfigs
osc create -f test/integration/fixtures/test-deployment-config.json
osc delete deploymentConfigs test-deployment-config
echo "deploymentConfigs: ok"

osc process -f examples/guestbook/template.json | osc apply -f -
echo "template+config: ok"

osc process -f examples/sample-app/application-template-dockerbuild.json | osc apply -f -
echo "buildConfig: ok"

started=$(osc start-build ruby-sample-build)
echo "start-build: ok"

osc cancel-build "${started}" --dump-logs --restart
echo "cancel-build: ok"

osc get minions,pods
