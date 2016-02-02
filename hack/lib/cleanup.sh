#!/bin/bash
#
# This library holds pluggable cleanup functions that can be installed to run in the EXIT handler for scripts.

# This library assumes $OS_ROOT is set
source "${OS_ROOT}/hack/util.sh"

# os::cleanup::dump_container_logs writes the logs from all Docker containers to the $LOG_DIR
# 
# Globals:
#  - LOG_DIR
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::dump_container_logs() {
    echo "[INFO] Dumping container logs to ${LOG_DIR}"
    local containers
    containers="$(docker ps -aq)"
    local container
    for container in ${containers}; do
        local container_name
        container_name=$(docker inspect -f "{{.Name}}" $container)
        # strip off leading /
        container_name=${container_name:1}
        if [[ "$container_name" =~ ^k8s_ ]]; then
            local pod_name
            pod_name=$(echo $container_name | awk 'BEGIN { FS="[_.]+" }; { print $4 }')
            container_name=${pod_name}-$(echo $container_name | awk 'BEGIN { FS="[_.]+" }; { print $2 }')
        fi
        docker logs "$container" >&"${LOG_DIR}/container-${container_name}.log"
    done
}

# os::cleanup::install_dump_container_logs installs the container log dump for the EXIT trap
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  - export OS_CLEANUP_DUMP_CONTAINER_LOGS
function os::cleanup::install_dump_container_logs() {
    export OS_CLEANUP_DUMP_CONTAINER_LOGS="true"
}

# os::cleanup::prune_artifacts deletes empty logs and truncates large logs in $ARTIFACT_DIR and $LOG_DIR
#
# Globals:
#  - ARTIFACT_DIR
#  - LOG_DIR
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::prune_artifacts() {
    echo "[INFO] Pruning artifacts"
    find "${ARTIFACT_DIR}" "${LOG_DIR}" -type f -name '*.log' \( -empty \) -delete

    local large_files="$( find "${ARTIFACT_DIR}" "${LOG_DIR}" -type f -name '*.log' \( -size +20M \) )"
    for file in ${large_files}; do
        cp "${file}" "${file}.tmp"
        echo "LOGFILE TOO LONG, PREVIOUS BYTES TRUNCATED. LAST 20M BYTES OF LOGFILE:" > "${file}"
        tail -c 20M "${file}.tmp" >> "${file}"
        rm "${file}.tmp"
    done
}

# os::cleanup::install_prune_artifacts installs the artifact prune for the EXIT trap
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  - export OS_CLEANUP_PRUNE_ARTIFACTS
function os::cleanup::install_prune_artifacts() {
    export OS_CLEANUP_PRUNE_ARTIFACTS="true"
}

# os::cleanup::dump_all_resources exports all OpenShift resources using the system admin credentials to a JSON artifact
#
# Globals:
#  - LOG_DIR
#  - ADMIN_KUBECONFIG
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::dump_all_resources() {
    if [[ ! -e "${ADMIN_KUBECONFIG:-}" ]]; then
        return 0
    fi

    echo "[INFO] Dumping all resources to ${LOG_DIR}/export_all.json"
    oc login -u system:admin -n default --config=${ADMIN_KUBECONFIG}
    oc export all --all-namespaces --raw -o json --config=${ADMIN_KUBECONFIG} > ${LOG_DIR}/export_all.json
}

# os::cleanup::install_dump_all_resources installs the OpenShift resource dump for the EXIT trap
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  - export OS_CLEANUP_DUMP_OPENSHIFT_RESOURCES
function os::cleanup::install_dump_all_resources() {
    export OS_CLEANUP_DUMP_OPENSHIFT_RESOURCES="true"
}

# os::cleanup::dump_etcd_contents dumps all contents of etcd to a JSON artifact
# 
# Globals:
#  - ARTIFACT_DIR
#  - API_SCHEME
#  - API_HOST
#  - ETCD_PORT
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::dump_etcd_contents() {
    echo "[INFO] Dumping etcd contents to ${ARTIFACT_DIR}/etcd_dump.json"
    set_curl_args 0 1
    curl -s ${clientcert_args} -L "${API_SCHEME}://${API_HOST}:${ETCD_PORT}/v2/keys/?recursive=true" > "${ARTIFACT_DIR}/etcd_dump.json"
}

# os::cleanup::install_dump_etcd_contents installs the etcd content dump for the EXIT trap
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  - export OS_CLEANUP_DUMP_ETCD_CONTENTS
function os::cleanup::install_dump_etcd_contents() {
    export OS_CLEANUP_DUMP_ETCD_CONTENTS="true"
}

# os::cleanup::dump_pprof_output dumps pprof output to an artifact
#
# Globals:
#  - ARTIFACT_DIR
#  - OS_ROOT
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::dump_pprof_output() {
    if go tool -n pprof >/dev/null 2>&1; then
        echo "[INFO] Dumping pprof output for the OpenShift binary"
        go tool pprof -text "${OS_ROOT}/_output/local/bin/$(os::util::host_platform)/openshift" "${OS_ROOT}/cpu.pprof" > "${ARTIFACT_DIR}/cpu.pprof.out"
    fi
}

# os::cleanup::install_dump_pprof_output installs pprof dump for the EXIT trap
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  - export OS_CLEANUP_DUMP_PPROF_OUTPUT
function os::cleanup::install_dump_pprof_output() {
    export OS_CLEANUP_DUMP_PPROF_OUTPUT="true"
}

# os::cleanup::kill_all_running_jobs kills all of the running jobs for this shell and all of their descendants
#
# Gloabls:
#  - SKIP_TEARDOWN
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::kill_all_running_jobs() {
    if [[ -n "${SKIP_TEARDOWN:-}" ]]; then
        return 0
    fi

    echo "[INFO] Killing all running jobs and their descendants"
    local running_jobs
    running_jobs="$( jobs -pr )"
    for job in ${running_jobs}; do
        local name
        name="$( ps --pid="${job}" --format=cmd --no-headers )"
        echo "[INFO] Killing top-level job ${job}: \"${name}\" and all descendants"
        os::cleanup::internal::kill_process_tree "${job}"
    done
}

# os::cleanup::internal::kill_process_tree recursively finds all processes in a tree and kills them, 
# starting with the root. This function will use elevated privileges if $USE_SUDO is set
#
# Globals:
#  - USE_SUDO
# Arguments:
#  - 1: PID of tree root
# Returns:
#  None
function os::cleanup::internal::kill_process_tree() {
    local root_pid=$1

    local child_pids
    child_pids="$( ps --ppid="${root_pid}" --format=pid --no-headers )"

    local sudo="${USE_SUDO:+sudo}"
    ${sudo} kill -SIGTERM "${root_pid}"

    for child_pid in ${child_pids}; do
        os::cleanup::internal::kill_process_tree "${child_pid}"
    done

    for (( i = 0; i < 10; i++ )); do
        sleep 0.5
        if ! ps --pid="${root_pid}" >/dev/null 2>&1; then
            return 0
        fi
    done

    local name
    name="$( ps --pid="${root_pid}" --format=cmd --no-headers )"
    echo "[WARNING] Giving up waiting for process ${root_pid}: \"${name}\" to exit after SIGKILL"
}

# os::cleanup::install_kill_all_running_jobs installs the job killing for the EXIT trap
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  - export OS_CLEANUP_KILL_RUNNING_JOBS
function os::cleanup::install_kill_all_running_jobs() {
    export OS_CLEANUP_KILL_RUNNING_JOBS="true"
}

# os::cleanup::kill_openshift_process_tree kills the OpenShift process and all of its descendants
#
# Globals:
#  - OS_PID
#  - SKIP_TEARDOWN
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::kill_openshift_process_tree() {
    if [[ -n "${SKIP_TEARDOWN:-}" ]]; then
        return 0
    fi

    echo "[INFO] Killing OpenShift process ${OS_PID} and all descendants"
    os::cleanup::internal::kill_process_tree "${OS_PID}"
}

# os::cleanup::install_kill_openshift_process_tree installs the OpenShift process tree killing for the EXIT trap
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  - export OS_CLEANUP_KILL_OPENSHIFT
function os::cleanup::install_kill_openshift_process_tree() {
    export OS_CLEANUP_KILL_OPENSHIFT="true"
}

# os::cleanup::stop_origin_container stops the OpenShift Origin container, when OpenShift is started as a Docker
# container instead of being run locally
#
# Globals:
#  - SKIP_TEARDOWN
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::stop_origin_container() {
    if [[ -n "${SKIP_TEARDOWN:-}" ]]; then
        return 0
    fi

    echo "[INFO] Stopping and removing the OpenShift Origin container"
    docker stop origin
    docker rm origin
}

# os::cleanup::install_stop_origin_container installs the OpenShift Origin container cleanup for the EXIT trap
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  - export OS_CLEANUP_STOP_ORIGIN_CONTAINER
function os::cleanup::install_stop_origin_container() {
    export OS_CLEANUP_STOP_ORIGIN_CONTAINER="true"
}

# os::cleanup::tear_down_k8s_containers stops and removes k8s Docker containers
#
# Globals:
#  - SKIP_TEARDOWN
#  - SKIP_IMAGE_CLEANUP
# Arguments:
#  None
# Returns:
#  None 
function os::cleanup::tear_down_k8s_containers() {
    if [[ -n "${SKIP_TEARDOWN:-}" ]]; then
        return 0
    fi

    local containers
    containers="$(docker ps | awk 'index($NF,"k8s_")==1 { print $1 }')"

    echo "[INFO] Stopping k8s Docker containers"
    for container in ${containers}; do
        docker stop "${container}"
    done

    if [[ -n "${SKIP_IMAGE_CLEANUP:-}" ]]; then
        return 0
    fi

    echo "[INFO] Removing k8s Docker containers"
    for container in ${containers}; do
        docker rm "${container}"
    done
}

# os::cleanup::install_tear_down_k8s_containers installs the k8s container tear down for the EXIT trap
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  - export OS_CLEANUP_TEARDOWN_K8S_CONTAINERS
function os::cleanup::install_tear_down_k8s_containers() {
    export OS_CLEANUP_TEARDOWN_K8S_CONTAINERS="true"
}

# os::cleanup::remove_scratch_image removes the test/scratchimage Docker image
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::remove_scratch_image() {
    echo "[INFO] Removing scratch Docker images"
    docker rmi test/scratchimage
}

# os::cleanup::install_remove_scratch_image installs the removal the test/scratchimage Docker image for the EXIT trap
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  - export OS_CLEANUP_REMOVE_SCRATCH_IMAGE
function os::cleanup::install_remove_scratch_image() {
    export OS_CLEANUP_REMOVE_SCRATCH_IMAGE="true"
}
