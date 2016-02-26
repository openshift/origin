#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT="${BASH_SOURCE%/*}/.."
OS_ROOT="$(readlink -ev "$OS_ROOT")"

cd "${OS_ROOT}"

# Setup environment
CLUSTER_NODES="${CLUSTER_NODES:-3}"
DOCKER_TAG="openshift/devhost:latest"

API_HOST="$(openshift start --print-ip)"
API_HOST="${API_HOST:-127.0.0.1}"
API_PORT="${API_PORT:-8443}"

export API_HOST API_PORT CLUSTER_NODES DOCKER_TAG

source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/lib/log.sh"
source "${OS_ROOT}/hack/lib/util/environment.sh"
source "${OS_ROOT}/hack/cmd_util.sh"

os::log::install_errexit
os::util::environment::setup_time_vars

function cleanup()
{
    local rc=$?
    trap - EXIT

    [[ "$rc" != 0 ]] &&
        printf "\n[FAIL] !!!!! Test Failed !!!!\n\n" ||
        printf "\n[INFO] Test Succeeded\n\n"

    dump_container_logs ||:

    if [[ -n "${ADMIN_KUBECONFIG-}" ]]; then
        echo "[INFO] Dumping all resources to ${LOG_DIR}/export_all.json"
        oc export all \
             --config="${ADMIN_KUBECONFIG}" \
            --all-namespaces \
            --raw \
            -o json > "${LOG_DIR}/export_all.json"
    fi

    if [[ -n "${ARTIFACT_DIR-}" ]]; then
        echo "[INFO] Dumping etcd contents to ${ARTIFACT_DIR}/etcd_dump.json"
        set_curl_args 0 1

        curl -s ${clientcert_args} \
            -L "${API_SCHEME}://${API_HOST}:${ETCD_PORT:-4001}/v2/keys/?recursive=true" \
            > "${ARTIFACT_DIR}/etcd_dump.json"
    fi

    if [[ -z "${SKIP_TEARDOWN-}" ]]; then
        node_list="$(seq -f 'node-%g' 1 ${CLUSTER_NODES}; echo master;)"

        echo
        echo "[INFO] Unpause docker containers..."
        for container in ${node_list}; do
            docker inspect -f '{{.State.Paused}}' "$container" |grep -qsFx false ||
                docker unpause "$container" ||:
        done

        echo
        echo "[INFO] Stop docker containers..."
        docker stop --time=2 $node_list ||:
        docker wait $node_list >/dev/null ||:

        echo
        echo "[INFO] Remove docker containers..."
        docker rm -vf $node_list ||:

        echo
        echo "[INFO] Remove '$NODE_X_BRIDGE_NAME' bridge"
        sudo ip link set dev "$NODE_X_BRIDGE_NAME" down
        sudo brctl delbr "$NODE_X_BRIDGE_NAME"
        echo
    fi

    delete_empty_logs ||:
    truncate_large_logs ||:

    ENDTIME=$(date +%s);

    echo "[INFO] Exiting"
    echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"

    exit "$rc"
}

if [[ "$(getenforce)" = "Enforcing" ]]; then
    echo "Error: This script is not compatible with SELinux enforcing mode." >&2
    exit 1
fi

trap "cleanup" EXIT HUP PIPE INT QUIT TERM

os::util::environment::setup_all_server_vars "test-multinode/"
reset_tmp_dir

echo "Logging to ${LOG_DIR}..."
os::log::start_system_logger

# Prevent user environment from colliding with the test setup
unset KUBECONFIG

# Check openshift version
openshift version

echo
echo "[INFO] Setup networking ..."
NODE_X_CIDR_PREFIX='172.16'
NODE_X_BRIDGE_NAME='cbr0'
NODE_X_BRIDGE_ADDR="${NODE_X_CIDR_PREFIX}.0.1/16"
NODE_X_POD_CIDR="${NODE_X_CIDR_PREFIX}.0.0/16"

os::cmd::expect_failure "ip -4 -oneline addr show up scope global |grep -F ' $NODE_X_CIDR_PREFIX.'"

os::cmd::expect_success "sudo brctl addbr '$NODE_X_BRIDGE_NAME'"
os::cmd::expect_success "sudo ip link set dev '$NODE_X_BRIDGE_NAME' mtu 1460"
os::cmd::expect_success "sudo ip addr add '$NODE_X_BRIDGE_ADDR' dev '$NODE_X_BRIDGE_NAME'"
os::cmd::expect_success "sudo ip link set dev '$NODE_X_BRIDGE_NAME' up"

# Specify the scheme and port for the listen address, but let the IP auto-discover.
# Set --public-master to localhost, for a stable link to the console.
echo
echo "[INFO] Create certificates for the OpenShift server to ${MASTER_CONFIG_DIR}"

# Find the same IP that openshift start will bind to.
# This allows access from pods that have to talk back to master
SERVER_HOSTNAME_LIST="${PUBLIC_MASTER_HOST},$(openshift start --print-ip),localhost"

openshift admin ca create-master-certs \
    --overwrite=false \
    --cert-dir="${MASTER_CONFIG_DIR}" \
    --hostnames="${SERVER_HOSTNAME_LIST}" \
    --master="${MASTER_ADDR}" \
    --public-master="${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}"

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

# FIXME
rm -rf -- "${SERVER_CONFIG_DIR}"/node-*

KUBELET_NODES=
KUBELET_URLS=

for i in `seq 1 ${CLUSTER_NODES}`; do
    NODE_HOST="127.0.0.$i"
    NODE_PORT=$((${KUBELET_PORT} + ${i} - 1))
    NODE_URL="${KUBELET_SCHEME}://${NODE_HOST}:${NODE_PORT}"

    KUBELET_NODES="${KUBELET_NODES} ${NODE_HOST}"
    KUBELET_URLS="${KUBELET_URLS} ${NODE_URL}"

    (
        echo
        KUBELET_HOST="${NODE_HOST}"

        os::util::environment::setup_all_server_vars "test-multinode/"

        openshift admin create-node-config \
          --listen="${KUBELET_SCHEME}://0.0.0.0:${NODE_PORT}" \
          --node-dir="${NODE_CONFIG_DIR}" \
          --node="${KUBELET_HOST}" \
          --hostnames="${KUBELET_HOST}" \
          --master="${MASTER_ADDR}" \
          --node-client-certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" \
          --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" \
          --signer-cert="${MASTER_CONFIG_DIR}/ca.crt" \
          --signer-key="${MASTER_CONFIG_DIR}/ca.key" \
          --signer-serial="${MASTER_CONFIG_DIR}/ca.serial.txt"

        os::util::sed "s/\\(bindAddress:.*\\):${KUBELET_PORT}\$/\\1:${NODE_PORT}/" \
            "${NODE_CONFIG_DIR}/node-config.yaml"

        cat >>"${NODE_CONFIG_DIR}/node-config.yaml"<<EOF
kubeletArguments:
  configure-cbr0:
    - "false"
EOF
        # validate config that was generated
        os::cmd::expect_success_and_text "openshift ex validate node-config ${NODE_CONFIG_DIR}/node-config.yaml" 'SUCCESS'
    )
done

export KUBELET_NODES

oadm create-bootstrap-policy-file --filename="${MASTER_CONFIG_DIR}/policy.json"

echo
echo "[INFO] Building containerized current host"
os::cmd::expect_success "docker build -t '${DOCKER_TAG}' '${OS_ROOT}/contrib/devhost-in-docker'"

function docker_run()
{
    echo
    echo "[INFO] Starting OpenShift containerized $1 server${2:+ ($2)}"

    sudo docker run \
        --detach \
        --privileged \
        --net=host \
        -e OPENSHIFT_ON_PANIC=crash \
        -e OPENSHIFT_DOCKER_BRIDGE="${NODE_X_BRIDGE_NAME}" \
        -e OPENSHIFT_NODE_CIDR="${NODE_X_CIDR_PREFIX}.$i.0/24" \
        -v /:/host:ro \
        -v "${OS_OUTPUT_BINPATH}/$(os::util::host_platform):/opt/bin:ro" \
        -v "${SERVER_CONFIG_DIR}:/opt/config/openshift.local.config:rw" \
        --name="$1${2:+-$2}" \
        --entrypoint="/openshift-$1" \
        "${DOCKER_TAG}"
}

# Start master
docker_run master

# Set variables for oc from now on
KUBERNETES_MASTER="${API_SCHEME}://${API_HOST}:${API_PORT}"
KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
CLUSTER_ADMIN_CONTEXT=$(oc config view --config=${KUBECONFIG} --flatten -o template --template='{{index . "current-context"}}')

export KUBERNETES_MASTER KUBECONFIG CLUSTER_ADMIN_CONTEXT

if [[ "${API_SCHEME}" == "https" ]]; then
    CURL_CA_BUNDLE="${MASTER_CONFIG_DIR}/ca.crt"
    CURL_CERT="${MASTER_CONFIG_DIR}/admin.crt"
    CURL_KEY="${MASTER_CONFIG_DIR}/admin.key"

    export CURL_CA_BUNDLE CURL_CERT CURL_KEY
fi

wait_for_url "${KUBERNETES_MASTER}/healthz" "apiserver: " 0.25 80
wait_for_url "${KUBERNETES_MASTER}/healthz/ready" "apiserver(ready): " 0.25 80

i=1
for url in ${KUBELET_URLS}; do
    docker_run node "$i"
    (wait_for_url "${url}/healthz" "[INFO] kubelet: " 0.5 60)
    i=$(($i+1))
done

# keep so we can reset kubeconfig after each test
cp -f -- "${KUBECONFIG}"{,.bak}

find "${OS_ROOT}/test/multinode" -name '*.sh' |
    grep -E "${1:-.*}" |
    sort -u |
while read testfile; do
    echo
    echo "++ ${testfile}"

    name="${testfile##*/}"
    name="${name%.sh}"

    export OC_PROJECT="testing-${name}"

    # switch back to a standard identity. This prevents individual tests from
    # changing contexts and messing up other tests
    echo
    echo "[INFO] Create a new project '${OC_PROJECT}' ..."
    oc project "${CLUSTER_ADMIN_CONTEXT}"
    oc new-project "${OC_PROJECT}"

    rc=
    "${testfile}" || rc=$?

    # Cleanup after testing
    oc delete all --all

    echo
    echo "[INFO] Remove project '${OC_PROJECT}' ..."
    oc project "${CLUSTER_ADMIN_CONTEXT}"
    oc delete project "${OC_PROJECT}"

    # since nothing ever gets deleted from kubeconfig, reset it
    cp -f -- "${KUBECONFIG}"{.bak,}

    [[ -z "$rc" ]] ||
        exit "$rc"
done

echo
echo "[INFO] Dumping all metrics to ${LOG_DIR}/metrics.log"
wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/metrics" "metrics: " 0.25 80 > "${LOG_DIR}/metrics.log"

echo
echo "${0##*/}: ok"
