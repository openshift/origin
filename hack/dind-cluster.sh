#!/bin/bash

# WARNING: The script modifies the host on which it is run.  It loads
# the openvwitch and br_netfilter modules, sets
# net.bridge.bridge-nf-call-iptables=0, and creates 2 loopback devices
# for each non-master node.  Consider creating dind clusters in a VM
# if this modification is undesirable:
#
#   OPENSHIFT_DIND_DEV_CLUSTER=1 vagrant up'
#
# Overview
# ========
#
# This script manages the lifecycle of an openshift dev cluster
# deployed to docker-in-docker containers.  Once 'start' has been used
# to successfully create a dind cluster, 'docker exec' can be used to
# access the created containers (named
# openshift-{master,node-1,node-2}) as if they were VMs.
#
# Dependencies
# ------------
#
# This script has been tested on Fedora 22, but should work on any
# release.  Docker is assumed to be installed.  At this time,
# boot2docker is not supported.
#
# SELinux
# -------
#
# Docker-in-docker's use of volumes is not compatible with selinux set
# to enforcing mode.  Set selinux to permissive or disable it
# entirely.
#
# Vagrant Dev Cluster
# -------------------
#
# At present the dind setup uses the same config (./openshift.local.*)
# as a vagrant-deployed cluster, so it is not possible to run both a
# vm-based dev cluster and a dind-based dev cluster from a given repo
# clone.  Until this is fixed, it is necessary to run only a vm or
# dind-based cluster at a time, or run them from separate repos.
#
# Bash Aliases
# ------------
#
# The following bash aliases are available in the cluster containers:
#
# oc-create-hello - create the 'hello' example app
# oc-less-log - invoke 'less' on the openshift daemon log (will target
#               the master or node log depending on the type of node)
# oc-tail-log - invoke tail on the openshift daemon log
#
# Process Management
# ------------------
#
# Due to docker-in-docker conflicting with systemd when running in a
# container, supervisord is used instead.  The 'supervisorctl' command
# is the equivalent of 'systemctl' and logs for managed processes can
# be found in /var/log/supervisor.
#
# Loopback Devices
# ----------------
#
# Due to the way docker-in-docker daemons interact with loopback
# devices, it is important to invoke 'dind-cluster.sh stop' on a
# running cluster instead of manually stopping the containers.  This
# ensures that the containerized docker daemons are gracefully
# shutdown and allowed to release their loopback devices before
# container shutdown.  If the daemons are not stopped before container
# shutdown, the associated loopback devices will be effectively
# unusable ('leaked') until a subsequent host reboot.  If enough
# loopback devices are leaked, cluster boot may not be possible since
# each openshift node running in a container depends on a docker
# daemon requiring 2 loopback devices.

set -o errexit
set -o nounset
set -o pipefail

source $(dirname "${BASH_SOURCE}")/dind/init.sh

function check-selinux() {
  if [ "$(getenforce)" = "Enforcing" ]; then
    >&2 echo "Error: This script is not compatible with SELinux enforcing mode."
    exit 1
  fi
}

IMAGE_REPO="${OS_DIND_IMAGE_REPO:-}"
IMAGE_TAG="${OS_DIND_IMAGE_TAG:-}"
DIND_IMAGE="${IMAGE_REPO}openshift/dind${IMAGE_TAG}"
BUILD_IMAGES="${OS_DIND_BUILD_IMAGES:-1}"

function build-image() {
  local build_root=$1
  local image_name=$2

  pushd "${build_root}" > /dev/null
    ${DOCKER_CMD} build -t "${image_name}" .
  popd > /dev/null
}

function build-images() {
  # Building images is done by default but can be disabled to allow
  # separation of image build from cluster creation.
  if [ "${BUILD_IMAGES}" = "1" ]; then
    echo "Building container images"
    if [ "${IMAGE_REPO}" != "" ]; then
      # Failure to cache is assumed to not be worth failing the build.
      ${DOCKER_CMD} pull "${DIND_IMAGE}" || true
    fi
    build-image "${ORIGIN_ROOT}/images/dind" "${DIND_IMAGE}"
    if [ "${IMAGE_REPO}" != "" ]; then
      ${DOCKER_CMD} push "${DIND_IMAGE}" || true
    fi
  fi
}

function get-docker-ip() {
  local cid=$1

  ${DOCKER_CMD} inspect --format '{{ .NetworkSettings.IPAddress }}' "${cid}"
}

# Ensure sufficient available loopback devices to support the
# indicated number of dind nodes.  Since it's not possible to create
# device nodes inside a container, this function needs to be called
# before launching a container that will run dind.
function ensure-loopback-for-dind() {
  local node_count=$1

  # Ensure extra loopback devices to minimize the potential for
  # contention.  Sometimes docker restarts during deployment don't
  # properly release the devices.
  local extra_loopback=4
  local loopback_per_node=2
  local required_free_loopback=$(( ( ${node_count} * ${loopback_per_node} ) + \
    ${extra_loopback} ))

  # Find the maximum index of existing loopback devices.
  local max_index=$(losetup | grep '/dev/loop' | tail -n 1 |
    sed -e 's|^/dev/loop\([0-9]\{1,\}\).*|\1|')
  if [ -z "${max_index}" ]; then
    max_index=0
  fi

  local requested_max_index=$(( ${max_index} + ${required_free_loopback} - 1))
  for i in $(eval echo "{${max_index}..${requested_max_index}}"); do
    if [ ! -e "/dev/loop${i}" ]; then
      sudo mknod "/dev/loop${i}" b 7 "${i}"
    fi
  done
}

function start() {
  # docker-in-docker's use of volumes is not compatible with SELinux
  check-selinux

  echo "Ensuring compatible host configuration"
  sudo modprobe openvswitch
  sudo modprobe br_netfilter || true
  sudo sysctl -w net.bridge.bridge-nf-call-iptables=0
  ensure-loopback-for-dind "${NUM_NODES}"
  mkdir -p "${CONFIG_ROOT}"

  build-images

  ## Create containers
  echo "Launching containers"
  local root_volume="-v ${ORIGIN_ROOT}:${DEPLOYED_ROOT}"
  local config_volume="-v ${CONFIG_ROOT}:${DEPLOYED_CONFIG_ROOT}"
  local base_run_cmd="${DOCKER_CMD} run -dt ${root_volume} ${config_volume}"

  local master_cid=$(${base_run_cmd} --privileged --name="${MASTER_NAME}" \
    --hostname="${MASTER_NAME}" "${DIND_IMAGE}")
  local master_ip=$(get-docker-ip "${master_cid}")

  local node_cids=()
  local node_ips=()
  for name in "${NODE_NAMES[@]}"; do
    local cid=$(${base_run_cmd} --privileged --name="${name}" \
      --hostname="${name}" "${DIND_IMAGE}")
    node_cids+=( "${cid}" )
    node_ips+=( $(get-docker-ip "${cid}") )
  done
  node_ips=$(os::util::join , ${node_ips[@]})

  ## Provision containers
  local args="${master_ip} ${NUM_NODES} ${node_ips} ${INSTANCE_PREFIX}"
  echo "Provisioning ${MASTER_NAME}"
  ${DOCKER_CMD} exec -t "${master_cid}" bash -c \
    "${SCRIPT_ROOT}/provision-master.sh ${args} ${MASTER_NAME} ${NETWORK_PLUGIN}"

  # Ensure that non-root users have read access to the configuration.
  # Security shouldn't be a concern for dind since it will only be
  # used for dev and test.
  find "${CONFIG_ROOT}" -type d -exec sudo chmod ga+rx {} \;
  find "${CONFIG_ROOT}" -type f -exec sudo chmod ga+r {} \;

  for (( i=0; i < ${#node_cids[@]}; i++ )); do
    local cid="${node_cids[$i]}"
    local name="${NODE_NAMES[$i]}"
    echo "Provisioning ${name}"
    ${DOCKER_CMD} exec "${cid}" bash -c \
      "${SCRIPT_ROOT}/provision-node.sh ${args} ${name}"
  done

  echo "Disabling scheduling for the sdn node"
  ${DOCKER_CMD} exec "${master_cid}" bash -cl \
    "osadm manage-node ${SDN_NODE_NAME} --schedulable=false > /dev/null"
}

function stop() {
  echo "Cleaning up docker-in-docker containers"
  local master_cid=$(${DOCKER_CMD} ps -qa --filter "name=${MASTER_NAME}")
  if [[ "${master_cid}" ]]; then
    ${DOCKER_CMD} rm -f "${master_cid}"
  fi

  local node_cids=$(${DOCKER_CMD} ps -qa --filter "name=${NODE_PREFIX}")
  if [[ "${node_cids}" ]]; then
    node_cids=(${node_cids//\n/ })
    for cid in "${node_cids[@]}"; do
      # Ensure that the nested docker daemon is stopped before attempting
      # container removal so associated loopback devices are properly
      # released.
      #
      # See: https://github.com/jpetazzo/dind/issues/19
      #
      local is_running=$(${DOCKER_CMD} inspect -f {{.State.Running}} "${cid}")
      if [ "${is_running}" = "true" ]; then
        ${DOCKER_CMD} exec -t "${cid}" "${SCRIPT_ROOT}/kill-docker.sh"
      fi
      ${DOCKER_CMD} rm -f "${cid}"
    done
  fi

  echo "Clearing configuration to avoid conflict with a future cluster"
  # The container will have created configuration as root
  sudo rm -rf ${CONFIG_ROOT}/openshift.local.*

  # Volume cleanup is not compatible with SELinux
  check-selinux

  # Cleanup orphaned volumes
  #
  # See: https://github.com/jpetazzo/dind#important-warning-about-disk-usage
  #
  echo "Cleaning up volumes used by docker-in-docker daemons"
  ${DOCKER_CMD} run -v /var/run/docker.sock:/var/run/docker.sock \
    -v /var/lib/docker:/var/lib/docker --rm martin/docker-cleanup-volumes

}

function test-net-e2e() {
  local focus_regex="${NETWORKING_E2E_FOCUS:-}"
  local skip_regex="${NETWORKING_E2E_SKIP:-}"

  if [ ! -d "${CONFIG_ROOT}" ]; then
    >&2 echo "Error: dind cluster not found.  To launch a cluster:"
    >&2 echo ""
    >&2 echo "    hack/dind-cluster.sh start"
    >&2 echo ""
    exit 1
  fi

  source ${ORIGIN_ROOT}/hack/util.sh
  source ${ORIGIN_ROOT}/hack/common.sh

  ensure_ginkgo_or_die

  os::build::extended

  os::util::run-net-extended-tests "${CONFIG_ROOT}" "${focus_regex}" \
    "${skip_regex}"
}


case "${1:-""}" in
  start)
    start
    ;;
  stop)
    stop
    ;;
  restart)
    stop
    start
    ;;
  build-images)
    BUILD_IMAGES=1
    build-images
    ;;
  test-net-e2e)
    test-net-e2e
    ;;
  *)
    echo "Usage: $0 {start|stop|restart|build-images|test-net-e2e}"
    exit 2
esac
