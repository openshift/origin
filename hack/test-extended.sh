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
export OS_DNS_PORT=$(go run ${OS_ROOT}/test/util/random_port/generate.go)
export ETCD_PORT=$(go run ${OS_ROOT}/test/util/random_port/generate.go)

DEFAULT_SERVER_IP=$(ifconfig | grep -Ev "(127.0.0.1|172.17.42.1)" | grep "inet " | head -n 1 | sed 's/adr://' | awk '{print $2}')

export OS_MASTER_ADDR=${DEFAULT_SERVER_IP}:${OS_MASTER_PORT}
export OS_DNS_ADDR=${DEFAULT_SERVER_IP}:${OS_DNS_PORT}
export KUBERNETES_MASTER="https://${OS_MASTER_ADDR}"

export TMPDIR=${TMPDIR:-/tmp}
export BASETMPDIR="${TMPDIR}/openshift-extended-tests"

# Remove all test artifacts from the previous run
rm -rf ${BASETMPDIR} && mkdir -p ${BASETMPDIR}

# Setup directories and certificates for 'curl'
export SERVER_CONFIG_DIR="${BASETMPDIR}/openshift.local.config"
export MASTER_CONFIG_DIR="${SERVER_CONFIG_DIR}/master"
export NODE_CONFIG_DIR="${SERVER_CONFIG_DIR}/node-127.0.0.1"
export CURL_CA_BUNDLE="${MASTER_CONFIG_DIR}/ca.crt"
export CURL_CERT="${MASTER_CONFIG_DIR}/admin.crt"
export CURL_KEY="${MASTER_CONFIG_DIR}/admin.key"
export KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
export OPENSHIFT_ON_PANIC=crash

cleanup() {
    set +e
    pid=$(cat ${BASETMPDIR}/server.pid 2>/dev/null)
    if [ ! -z "$pid" ]; then
      server_pids=$(pgrep -P $pid)
      kill $server_pids $(cat ${BASETMPDIR}/server.pid) ${ETCD_PID}
    fi
    rm -rf ${ETCD_DIR-}
    echo "[INFO] Cleanup complete"
}

# TODO: There is a lot of code shared between this test launcher and e2e test
#       launcher.
start_server() {
  mkdir -p ${BASETMPDIR}/volumes
  ALL_IP_ADDRESSES=`ifconfig | grep "inet " | sed 's/adr://' | awk '{print $2}'`
  SERVER_HOSTNAME_LIST="${DEFAULT_SERVER_IP},localhost"
  while read -r IP_ADDRESS; do
    SERVER_HOSTNAME_LIST="${SERVER_HOSTNAME_LIST},${IP_ADDRESS}"
  done <<< "${ALL_IP_ADDRESSES}"

  echo "[INFO] Create certificates for the OpenShift master"
  env "PATH=${PATH}" openshift admin create-master-certs \
    --overwrite=false \
    --cert-dir="${MASTER_CONFIG_DIR}" \
    --hostnames="${SERVER_HOSTNAME_LIST}" \
    --master="https://${OS_MASTER_ADDR}" \
    --public-master="https://${OS_MASTER_ADDR}"

  echo "[INFO] Create certificates for the OpenShift node"
  env "PATH=${PATH}" openshift admin create-node-config \
    --listen="https://0.0.0.0:10250" \
    --node-dir="${NODE_CONFIG_DIR}" \
    --node="127.0.0.1" \
    --hostnames="${SERVER_HOSTNAME_LIST}" \
    --master="https://${OS_MASTER_ADDR}" \
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
    --listen="https://0.0.0.0:${OS_MASTER_PORT}" \
    --public-master="https://${OS_MASTER_ADDR}" \
    --etcd="http://127.0.0.1:${ETCD_PORT}" \
    --hostname="127.0.0.1" \
    --volume-dir="${BASETMPDIR}/volumes" \
    --master="https://${OS_MASTER_ADDR}" \
    --latest-images

  echo "[INFO] Starting OpenShift ..."
  sudo env "PATH=${PATH}" openshift start \
    --master-config=${MASTER_CONFIG_DIR}/master-config.yaml \
    --node-config=${NODE_CONFIG_DIR}/node-config.yaml \
    --loglevel=${VERBOSE:-3} &> ${BASETMPDIR}/server.log &
  echo -n $! > ${BASETMPDIR}/server.pid
}

start_docker_registry() {
  mkdir -p ${BASETMPDIR}/.registry
  echo "[INFO] Creating Router ..."
  oadm router --create --credentials="${KUBECONFIG}" \
    --images='openshift/origin-${component}:latest' &>/dev/null

  echo "[INFO] Creating Registry ..."
  oadm registry --create --credentials="${KUBECONFIG}" \
    --mount-host="${BASETMPDIR}/.registry" \
    --images='openshift/origin-${component}:latest' &>/dev/null
}

push_to_registry() {
  local image=$1
  local registry=$2
  echo "[INFO] Caching $image to $registry"
  ( docker tag $image "${registry}/${image}" && \
    docker push "${registry}/${image}" \
  ) &>/dev/null
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
wait_for_url "https://${OS_MASTER_ADDR}/api/v1/nodes/127.0.0.1" "" 0.25 80 >/dev/null

# Start the Docker registry (172.30.17.101:5000)
start_docker_registry

wait_for_command '[[ "$(oc get endpoints docker-registry -t "{{ if .endpoints}}{{ len .endpoints }}{{ else }}0{{ end }}" 2>/dev/null || echo "0")" != "0" ]]' $((5*TIME_MIN))

REGISTRY_ADDR=$(oc get --output-version=v1 --template="{{ .spec.portalIP }}:{{.port }}" \
  service docker-registry)
echo "[INFO] Verifying the docker-registry is up at ${REGISTRY_ADDR}"
wait_for_url_timed "http://${REGISTRY_ADDR}" "" $((2*TIME_MIN))

# TODO: We need to pre-push the images that we use for builds to avoid getting
#       "409 - Image already exists" during the 'push' when the Build finishes.
#       This is because Docker Registry cannot handle parallel pushes.
#       See: https://github.com/docker/docker-registry/issues/537
push_to_registry "openshift/ruby-20-centos7" $REGISTRY_ADDR
push_to_registry "openshift/origin-custom-docker-builder" $REGISTRY_ADDR

export REGISTRY_ADDR

[ ! -z "${DEBUG-}" ] && set +e

# Run all extended tests cases
while true; do
  echo "[INFO] Starting extended tests ..."
  time OS_TEST_PACKAGE="test/extended" OS_TEST_TAGS="extended" OS_TEST_NAMESPACE="extended" ${OS_ROOT}/hack/test-integration.sh $@
  if [ ! -z "${DEBUG-}" ]; then
    read -p "Do you want to re-run the test cases? " yn
    case $yn in
        [Nn]* ) exit;;
        * ) echo "Please answer yes or no.";;
    esac
  fi
done
