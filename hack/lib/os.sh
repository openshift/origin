#!/bin/bash
#
# This library contains functions related to configuring, starting, stopping, and cleaning up OpenShift
# apiservers, OpenShift masters, and standalone etcds.

# This script assumes OS_ROOT is set
source "${OS_ROOT}/hack/lib/util/misc.sh"
source "${OS_ROOT}/hack/lib/cleanup.sh"
source "${OS_ROOT}/hack/util.sh"

# os::configure_server creates and writes the master certificates, node configuration, bootstrap policy,
# and master configuration for an OpenShift instance.
#
# Globals:
#  - ALL_IP_ADDRESSES
#  - PUBLIC_MASTER_HOST
#  - MASTER_CONFIG_DIR
#  - MASTER_ADDR
#  - API_SCHEME
#  - PUBLIC_MASTER_HOST
#  - API_PORT
#  - KUBELET_SCHEME
#  - KUBELET_BIND_HOST
#  - KUBELET_PORT
#  - NODE_CONFIG_DIR
#  - KUBELET_HOST
#  - API_BIND_HOST
#  - VOLUME_DIR
#  - ETCD_DATA_DIR
#  - ETCD_PORT
#  - ETCD_PEER_PORT
#  - SERVER_CONFIG_DIR
#  - USE_IMAGES
#  - USE_SUDO
# Arguments:
#  None
# Returns:
#  - export ADMIN_KUBECONFIG
#  - export CLUSTER_ADMIN_CONTEXT
#  - export ALL_IP_ADDRESSES
#  - export SERVER_HOSTNAME_LIST
function os::configure_server() {
    # find the same IP that openshift start will bind to.   This allows access from pods that have to talk back to master
    if [[ -z "${ALL_IP_ADDRESSES-}" ]]; then
        ALL_IP_ADDRESSES="$(openshift start --print-ip)"
        SERVER_HOSTNAME_LIST="${PUBLIC_MASTER_HOST},localhost,172.30.0.1"
                SERVER_HOSTNAME_LIST="${SERVER_HOSTNAME_LIST},kubernetes.default.svc.cluster.local,kubernetes.default.svc,kubernetes.default,kubernetes"
                SERVER_HOSTNAME_LIST="${SERVER_HOSTNAME_LIST},openshift.default.svc.cluster.local,openshift.default.svc,openshift.default,openshift"

        while read -r IP_ADDRESS
        do
            SERVER_HOSTNAME_LIST="${SERVER_HOSTNAME_LIST},${IP_ADDRESS}"
        done <<< "${ALL_IP_ADDRESSES}"

        export ALL_IP_ADDRESSES
        export SERVER_HOSTNAME_LIST
    fi

    echo "[INFO] Creating certificates for the OpenShift server"
    openshift admin ca create-master-certs \
        --overwrite=false \
        --cert-dir="${MASTER_CONFIG_DIR}" \
        --hostnames="${SERVER_HOSTNAME_LIST}" \
        --master="${MASTER_ADDR}" \
        --public-master="${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}"

    echo "[INFO] Creating OpenShift node config"
    openshift admin create-node-config \
        --listen="${KUBELET_SCHEME}://${KUBELET_BIND_HOST}:${KUBELET_PORT}" \
        --node-dir="${NODE_CONFIG_DIR}" \
        --node="${KUBELET_HOST}" \
        --hostnames="${KUBELET_HOST}" \
        --master="${MASTER_ADDR}" \
        --node-client-certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" \
        --certificate-authority="${MASTER_CONFIG_DIR}/ca.crt" \
        --signer-cert="${MASTER_CONFIG_DIR}/ca.crt" \
        --signer-key="${MASTER_CONFIG_DIR}/ca.key" \
        --signer-serial="${MASTER_CONFIG_DIR}/ca.serial.txt"

    echo "[INFO] Creating OpenShift bootstrap policy file"
    oadm create-bootstrap-policy-file --filename="${MASTER_CONFIG_DIR}/policy.json"

    echo "[INFO] Creating OpenShift config"
    openshift start \
        --write-config=${SERVER_CONFIG_DIR} \
        --create-certs=false \
        --listen="${API_SCHEME}://${API_BIND_HOST}:${API_PORT}" \
        --master="${MASTER_ADDR}" \
        --public-master="${API_SCHEME}://${PUBLIC_MASTER_HOST}:${API_PORT}" \
        --hostname="${KUBELET_HOST}" \
        --volume-dir="${VOLUME_DIR}" \
        --etcd-dir="${ETCD_DATA_DIR}" \
        --images="${USE_IMAGES}"


    # Don't try this at home.  We don't have flags for setting etcd ports in the config, but we want deconflicted ones.  Use sed to replace defaults in a completely unsafe way
    os::util::sed "s/:4001$/:${ETCD_PORT}/g" ${SERVER_CONFIG_DIR}/master/master-config.yaml
    os::util::sed "s/:7001$/:${ETCD_PEER_PORT}/g" ${SERVER_CONFIG_DIR}/master/master-config.yaml


    # Make oc use ${MASTER_CONFIG_DIR}/admin.kubeconfig, and ignore anything in the running user's $HOME dir
    export ADMIN_KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
    export CLUSTER_ADMIN_CONTEXT=$(oc config view --config=${ADMIN_KUBECONFIG} --flatten -o template --template='{{index . "current-context"}}')
    local sudo="${USE_SUDO:+sudo}"
    ${sudo} chmod -R a+rwX "${ADMIN_KUBECONFIG}"
    echo "[INFO] To debug: export KUBECONFIG=$ADMIN_KUBECONFIG"
}

# os::start_server starts the OpenShift server, exports its PID and waits until endpoints are available
#
# Globals:
#  - USE_SUDO
#  - LOG_DIR
#  - ARTIFACT_DIR
#  - VOLUME_DIR
#  - SERVER_CONFIG_DIR
#  - USE_IMAGES
#  - MASTER_ADDR
#  - MASTER_CONFIG_DIR
#  - NODE_CONFIG_DIR
#  - API_SCHEME
#  - API_HOST
#  - API_PORT
#  - KUBELET_SCHEME
#  - KUBELET_HOST
#  - KUBELET_PORT
# Arguments:
#  None
# Returns:
#  - export OS_PID
function os::start_server {
    os::internal::install_server_cleanup

    local sudo="${USE_SUDO:+sudo}"

    echo "[INFO] `openshift version`"
    echo "[INFO] Server logs will be at:    ${LOG_DIR}/openshift.log"
    echo "[INFO] Test artifacts will be in: ${ARTIFACT_DIR}"
    echo "[INFO] Volumes dir is:            ${VOLUME_DIR}"
    echo "[INFO] Config dir is:             ${SERVER_CONFIG_DIR}"
    echo "[INFO] Using images:              ${USE_IMAGES}"
    echo "[INFO] MasterIP is:               ${MASTER_ADDR}"

    mkdir -p ${LOG_DIR}

    echo "[INFO] Scan of OpenShift related processes already up via ps -ef  | grep openshift : "
    ps -ef | grep openshift
    echo "[INFO] Starting OpenShift server"
    ${sudo} env "PATH=${PATH}" OPENSHIFT_PROFILE="${OPENSHIFT_PROFILE:-web}" OPENSHIFT_ON_PANIC=crash openshift start \
        --master-config=${MASTER_CONFIG_DIR}/master-config.yaml \
        --node-config=${NODE_CONFIG_DIR}/node-config.yaml \
        --loglevel=4 \
        &>"${LOG_DIR}/openshift.log" &
    export OS_PID=$!

    echo "[INFO] OpenShift server start at: "
    date

    wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
    wait_for_url "${KUBELET_SCHEME}://${KUBELET_HOST}:${KUBELET_PORT}/healthz" "[INFO] kubelet: " 0.5 60
    wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 80
    wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/api/v1/nodes/${KUBELET_HOST}" "apiserver(nodes): " 0.25 80

    echo "[INFO] OpenShift server health checks done at: "
    date
}

# os::internal::install_server_cleanup installs all of the necessary cleanup modules for scripts that start OpenShift servers
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::internal::install_server_cleanup() {
    os::util::install_describe_return_code
    os::cleanup::install_dump_container_logs
    os::cleanup::install_dump_all_resources
    os::cleanup::install_dump_etcd_contents
    os::cleanup::install_kill_openshift_process_tree
    os::cleanup::install_kill_all_running_jobs
    os::cleanup::install_tear_down_k8s_containers
    os::cleanup::install_prune_artifacts
}

# os::start_master starts the OpenShift master, exports its PID and waits until endpoints are available
#
# Globals:
#  - USE_SUDO
#  - LOG_DIR
#  - ARTIFACT_DIR
#  - SERVER_CONFIG_DIR
#  - USE_IMAGES
#  - MASTER_ADDR
#  - MASTER_CONFIG_DIR
#  - API_SCHEME
#  - API_HOST
#  - API_PORT
# Arguments:
#  None
# Returns:
#  - export OS_PID
function os::start_master {
    os::internal::install_master_cleanup

    local sudo="${USE_SUDO:+sudo}"

    echo "[INFO] `openshift version`"
    echo "[INFO] Server logs will be at:    ${LOG_DIR}/openshift.log"
    echo "[INFO] Test artifacts will be in: ${ARTIFACT_DIR}"
    echo "[INFO] Config dir is:             ${SERVER_CONFIG_DIR}"
    echo "[INFO] Using images:              ${USE_IMAGES}"
    echo "[INFO] MasterIP is:               ${MASTER_ADDR}"

    mkdir -p ${LOG_DIR}

    echo "[INFO] Scan of OpenShift related processes already up via ps -ef  | grep openshift : "
    ps -ef | grep openshift
    echo "[INFO] Starting OpenShift server"
    ${sudo} env "PATH=${PATH}" OPENSHIFT_PROFILE="${OPENSHIFT_PROFILE:-web}" OPENSHIFT_ON_PANIC=crash openshift start master \
        --config=${MASTER_CONFIG_DIR}/master-config.yaml \
        --loglevel=4 \
        &>"${LOG_DIR}/openshift.log" &
    export OS_PID=$!

    echo "[INFO] OpenShift server start at: "
    date
    
    wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
    wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 80
    
    echo "[INFO] OpenShift server health checks done at: "
    date
}

# os::internal::install_master_cleanup installs all of the necessary cleanup modules for scripts that start OpenShift masters
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::internal::install_master_cleanup() {
    os::util::install_describe_return_code
    os::cleanup::install_dump_all_resources
    os::cleanup::install_dump_etcd_contents
    os::cleanup::install_kill_openshift_process_tree
    os::cleanup::install_kill_all_running_jobs
    os::cleanup::install_prune_artifacts
}

# os::start_container starts the OpenShift Origin Docker container
#
# Globals:
#  - VOLUME_DIR
#  - TAG
#  - USE_IMAGES
#  - BASETMPDIR
#  - MASTER_CONFIG_DIR
#  - KUBELET_SCHEME
#  - KUBELET_HOST
#  - KUBELET_PORT
#  - API_SCHEME
#  - API_HOST
#  - API_PORT
# Arguments:
#  None
# Returns:
#  - export ADMIN_KUBECONFIG
#  - export CLUSTER_ADMIN_CONTEXT
#  - export KUBECONFIG
function os::start_container() {
    os::internal::install_containerized_cleanup

    echo "[INFO] `openshift version`"
    echo "[INFO] Using images:                          ${USE_IMAGES}"

    mkdir -p /tmp/openshift-e2e/etcd || true

    echo "[INFO] Starting OpenShift containerized server"
    sudo docker run -d --name="origin" \
        --privileged --net=host --pid=host \
        -v /:/rootfs:ro -v /var/run:/var/run:rw -v /sys:/sys:ro -v /var/lib/docker:/var/lib/docker:rw \
        -v "${VOLUME_DIR}:${VOLUME_DIR}" -v /tmp/openshift-e2e/etcd:/var/lib/origin/openshift.local.etcd:rw \
        "openshift/origin:${TAG}" start --loglevel=4 --volume-dir=${VOLUME_DIR} --images="${USE_IMAGES}"


    # the CA is in the container, log in as a different cluster admin to run the test
    CURL_EXTRA="-k"
    wait_for_url "https://localhost:8443/healthz/ready" "apiserver(ready): " 0.25 160

    IMAGE_WORKING_DIR=/var/lib/origin
    docker cp origin:${IMAGE_WORKING_DIR}/openshift.local.config ${BASETMPDIR}

    export ADMIN_KUBECONFIG="${MASTER_CONFIG_DIR}/admin.kubeconfig"
    export CLUSTER_ADMIN_CONTEXT=$(oc config view --config=${ADMIN_KUBECONFIG} --flatten -o template --template='{{index . "current-context"}}')
    sudo chmod -R a+rwX "${ADMIN_KUBECONFIG}"
    export KUBECONFIG="${ADMIN_KUBECONFIG}"
    echo "[INFO] To debug: export KUBECONFIG=$ADMIN_KUBECONFIG"


    wait_for_url "${KUBELET_SCHEME}://${KUBELET_HOST}:${KUBELET_PORT}/healthz" "[INFO] kubelet: " 0.5 60
    wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz" "apiserver: " 0.25 80
    wait_for_url "${API_SCHEME}://${API_HOST}:${API_PORT}/healthz/ready" "apiserver(ready): " 0.25 80
}

# os::internal::install_containerized_cleanup installs all of the necessary cleanup modules for scripts that start OpenShift in a container
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::internal::install_containerized_cleanup() {
    os::util::install_describe_return_code
    os::cleanup::install_dump_container_logs
    os::cleanup::install_dump_all_resources
    os::cleanup::install_dump_etcd_contents
    os::cleanup::install_stop_origin_container
    os::cleanup::install_kill_all_running_jobs
    os::cleanup::install_tear_down_k8s_containers
    os::cleanup::install_prune_artifacts
}
