#!/bin/bash

set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..

source ${OS_ROOT}/hack/util.sh
source ${OS_ROOT}/hack/common.sh

TIME_SEC=1000
TIME_MIN=$((60 * $TIME_SEC))

# TODO: Randomize these ports
export OS_MASTER_PORT=$(go run ${OS_ROOT}/test/util/random_port/generate.go)
export OS_ASSETS_PORT=$(go run ${OS_ROOT}/test/util/random_port/generate.go)
export OS_DNS_PORT=$(go run ${OS_ROOT}/test/util/random_port/generate.go)
export ETCD_PORT=$(go run ${OS_ROOT}/test/util/random_port/generate.go)

DEFAULT_SERVER_IP=$(ifconfig | grep -Ev "(127.0.0.1|172.17.42.1)" | grep "inet " | head -n 1 | awk '{print $2}')

export OS_MASTER_ADDR=${DEFAULT_SERVER_IP}:${OS_MASTER_PORT}
export OS_ASSETS_ADDR=${DEFAULT_SERVER_IP}:${OS_ASSETS_PORT}
export OS_DNS_ADDR=${DEFAULT_SERVER_IP}:${OS_DNS_PORT}
export KUBERNETES_MASTER="https://${OS_MASTER_ADDR}"

export TMPDIR=${TMPDIR:-/tmp}
export BASETMPDIR="${TMPDIR}/openshift-extended-tests"

# Remove all test artifacts from the previous run
rm -rf ${BASETMPDIR} && mkdir -p ${BASETMPDIR}

# Setup directories and certificates for 'curl'
export CERT_DIR="${BASETMPDIR}/cert"
export CURL_CA_BUNDLE="${CERT_DIR}/ca/cert.crt"
export CURL_CERT="${CERT_DIR}/admin/cert.crt"
export CURL_KEY="${CERT_DIR}/admin/key.key"
export KUBECONFIG="${CERT_DIR}/admin/.kubeconfig"
export OPENSHIFT_ON_PANIC=crash

cleanup() {
    set +e
    server_pids=$(pgrep -P $(cat ${BASETMPDIR}/server.pid))
    kill $server_pids $(cat ${BASETMPDIR}/server.pid) ${ETCD_PID}
    rm -rf ${ETCD_DIR}
    echo "[INFO] Cleanup complete"
}

# TODO: There is a lot of code shared between this test launcher and e2e test
#       launcher.
start_server() {
  mkdir -p ${BASETMPDIR}/volumes
  ALL_IP_ADDRESSES=`ifconfig | grep "inet " | awk '{print $2}'`
  SERVER_HOSTNAME_LIST="${DEFAULT_SERVER_IP},localhost"
  while read -r IP_ADDRESS; do
    SERVER_HOSTNAME_LIST="${SERVER_HOSTNAME_LIST},${IP_ADDRESS}"
  done <<< "${ALL_IP_ADDRESSES}"

  echo "[INFO] Create certificates for the OpenShift server"
  sudo env "PATH=${PATH}" openshift admin create-all-certs \
  --overwrite=false \
  --cert-dir="${CERT_DIR}" \
  --hostnames="${SERVER_HOSTNAME_LIST}" \
  --nodes="127.0.0.1" \
  --master="https://${OS_MASTER_ADDR}" \
  --public-master="https://${OS_MASTER_ADDR}"

  echo "[INFO] Starting OpenShift server"
  sudo env "PATH=${PATH}" openshift start \
    --listen="https://0.0.0.0:${OS_MASTER_PORT}" \
    --public-master="https://${OS_MASTER_ADDR}" \
    --etcd="http://127.0.0.1:${ETCD_PORT}" \
    --hostname="127.0.0.1" \
    --create-certs=false \
    --cert-dir="${CERT_DIR}" \
    --volume-dir="${BASETMPDIR}/volumes" \
    --master="https://${OS_MASTER_ADDR}" \
    --latest-images \
    --loglevel=${VERBOSE:-3} &> ${BASETMPDIR}/server.log &
  echo -n $! > ${BASETMPDIR}/server.pid
}

start_docker_registry() {
  mkdir -p ${BASETMPDIR}/.registry
  echo "[INFO] Creating default Router"
  openshift ex router --create --credentials="${KUBECONFIG}" \
    --images='openshift/origin-${component}:latest' &>/dev/null

  echo "[INFO] Creating Docker Registry"
  openshift ex registry --create --credentials="${KUBECONFIG}" \
    --mount-host="${BASETMPDIR}/.registry" \
    --images='openshift/origin-${component}:latest' &>/dev/null
}

# Go to the top of the tree.
cd "${OS_ROOT}"

trap cleanup EXIT SIGINT

# Start the Etcd server
echo "[INFO] Starting etcd server (127.0.0.1:${ETCD_PORT})"
start_etcd
export ETCD_STARTED="1"

# Start OpenShift sever that will be common for all extended test cases
start_server

# Wait for the API server to come up
wait_for_url_timed "https://${OS_MASTER_ADDR}/healthz" "" 90*TIME_SEC >/dev/null
wait_for_url_timed "https://${OS_MASTER_ADDR}/osapi" "" 90*TIME_SEC >/dev/null
wait_for_url "https://${OS_MASTER_ADDR}/api/v1beta1/minions/127.0.0.1" "" 0.25 80 >/dev/null

# Start the Docker registry (172.30.17.101:5000)
start_docker_registry

wait_for_command '[[ "$(osc get endpoints docker-registry -t "{{ if .endpoints}}{{ len .endpoints }}{{ else }}0{{ end }}" 2>/dev/null || echo "0")" != "0" ]]' $((5*TIME_MIN))

REGISTRY_ADDR=$(osc get --output-version=v1beta1 --template="{{ .portalIP }}:{{.port }}" \
  service docker-registry)
echo "[INFO] Verifying the docker-registry is up at ${REGISTRY_ADDR}"
wait_for_url_timed "http://${REGISTRY_ADDR}" "" $((2*TIME_MIN))

# TODO: We need to pre-push the images that we use for builds to avoid getting
#       "409 - Image already exists" during the 'push' when the Build finishes.
#       This is because Docker Registry cannot handle parallel pushes.
#       See: https://github.com/docker/docker-registry/issues/537
echo "[INFO] Pushing openshift/ruby-20-centos7 image to ${REGISTRY_ADDR}"
docker tag openshift/ruby-20-centos7 ${REGISTRY_ADDR}/openshift/ruby-20-centos7
docker push ${REGISTRY_ADDR}/openshift/ruby-20-centos7 &>/dev/null

# Run all extended tests cases
echo "[INFO] Starting extended tests"
OS_TEST_PACKAGE="test/extended" OS_TEST_TAGS="extended" OS_TEST_NAMESPACE="extended" ${OS_ROOT}/hack/test-integration.sh $@
