#!/bin/bash

# WARNING: The script modifies the host on which it is run.  It loads
# the openvwitch and br_netfilter modules and sets
# net.bridge.bridge-nf-call-iptables=0.  Consider creating dind
# clusters in a VM if this modification is undesirable:
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
# This script has been tested on Fedora 21, but should work on any
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
# OpenShift Configuration
# -----------------------
#
# By default, a dind openshift cluster stores its configuration
# (openshift.local.*) in /tmp/openshift-dind-cluster/openshift.  Since
# configuration is stored in a different location than a
# vagrant-deployed cluster (which stores configuration in the root of
# the origin tree), vagrant and dind clusters can run simultaneously
# without conflict.  It's also possible to run multiple dind clusters
# simultaneously by overriding the instance prefix.  The following
# command would ensure configuration was stored at
# /tmp/openshift-dind/cluster/my-cluster:
#
#    OPENSHIFT_INSTANCE_PREFIX=my-cluster hack/dind-cluster.sh [command]
#
# It is also possible to specify an entirely different configuration path:
#
#    OPENSHIFT_CONFIG_ROOT=[path] hack/dind-cluster.sh [command]
#
# Running Tests
# -------------
#
# This script includes a shortcut for running the networking e2e
# tests.  The test-net-e2e command will run the extended networking
# tests against an already running dind cluster:
#
#     hack/dind-cluster.sh test-net-e2e
#

set -o errexit
set -o nounset
set -o pipefail

DIND_MANAGEMENT_SCRIPT=true

source $(dirname "${BASH_SOURCE}")/../contrib/vagrant/provision-config.sh

DOCKER_CMD=${DOCKER_CMD:-"sudo docker"}

# Override the default CONFIG_ROOT path with one that is
# cluster-specific.
CONFIG_ROOT=${OPENSHIFT_CONFIG_ROOT:-/tmp/openshift-dind-cluster/${INSTANCE_PREFIX}}

DEPLOYED_CONFIG_ROOT="/config"

DEPLOYED_ROOT="/data"

SCRIPT_ROOT="${DEPLOYED_ROOT}/contrib/vagrant"

function check-selinux() {
  if [ "$(getenforce)" = "Enforcing" ]; then
    >&2 echo "Error: This script is not compatible with SELinux enforcing mode."
    exit 1
  fi
}

IMAGE_REPO="${OPENSHIFT_DIND_IMAGE_REPO:-}"
IMAGE_TAG="${OPENSHIFT_DIND_IMAGE_TAG:-}"
DIND_IMAGE="${IMAGE_REPO}openshift/dind${IMAGE_TAG}"
BUILD_IMAGES="${OPENSHIFT_DIND_BUILD_IMAGES:-1}"

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

function start() {
  # docker-in-docker's use of volumes is not compatible with SELinux
  check-selinux

  # TODO(marun) - perform these operations in a container for boot2docker compat
  echo "Ensuring compatible host configuration"
  sudo modprobe openvswitch
  sudo modprobe br_netfilter || true
  sudo sysctl -w net.bridge.bridge-nf-call-iptables=0
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
  echo "Configured network plugin: ${NETWORK_PLUGIN}"
  local args="${master_ip} ${NODE_COUNT} ${node_ips} ${INSTANCE_PREFIX} \
-n '${NETWORK_PLUGIN}'"
  echo "Provisioning ${MASTER_NAME}"
  ${DOCKER_CMD} exec -t "${master_cid}" bash -c \
    "${SCRIPT_ROOT}/provision-master.sh ${args} -c ${DEPLOYED_CONFIG_ROOT}"

  # Ensure that all users (e.g. outside the container) have read-write
  # access to the openshift configuration.  Security shouldn't be a
  # concern for dind since it should only be used for dev and test.
  local openshift_config_path="${CONFIG_ROOT}/openshift.local.config"
  find "${openshift_config_path}" -exec sudo chmod ga+rw {} \;
  find "${openshift_config_path}" -type d -exec sudo chmod ga+x {} \;

  for (( i=0; i < ${#node_cids[@]}; i++ )); do
    local cid="${node_cids[$i]}"
    local name="${NODE_NAMES[$i]}"
    echo "Provisioning ${name}"
    ${DOCKER_CMD} exec "${cid}" bash -c \
      "${SCRIPT_ROOT}/provision-node.sh ${args} -i ${i} -c \
${DEPLOYED_CONFIG_ROOT}"
  done

  os::util::disable-sdn-node "${master_cid}" "${SDN_NODE_NAME}"
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
      ${DOCKER_CMD} rm -f "${cid}"
    done
  fi

  echo "Cleaning up cluster configuration"
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

  go get github.com/onsi/ginkgo/ginkgo

  os::build::setup_env
  go test -c ./test/extended/networking -o ${OS_OUTPUT_BINPATH}/networking.test

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
  config-host)
    os::util::set-os-env "${ORIGIN_ROOT}" "${CONFIG_ROOT}"
    ;;
  *)
    echo "Usage: $0 {start|stop|restart|build-images|test-net-e2e|config-host}"
    exit 2
esac
