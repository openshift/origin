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
# Suggested Workflow
# ------------------
#
# When making changes to the deployment of a dind cluster or making
# breaking golang changes, the 'restart' command will ensure that an
# existing cluster is cleaned up before deploying a new cluster.
#
# When only making non-breaking changes to golang code, the 'redeploy'
# command avoids restarting the cluster.  'redeploy' rebuilds the
# openshift binaries and deploys them to the existing cluster.
#
# Running Tests
# -------------
#
# The extended tests can be run against a dind cluster as follows:
#
#     OPENSHIFT_CONFIG_ROOT=dind test/extended/networking.sh

set -o errexit
set -o nounset
set -o pipefail

DIND_MANAGEMENT_SCRIPT=true

source $(dirname "${BASH_SOURCE}")/../contrib/vagrant/provision-config.sh

# Enable xtrace for container script invocations if it is enabled
# for this script.
BASH_CMD=
if [ "$(set | grep xtrace)" ]; then
    BASH_CMD="bash -x"
fi

DOCKER_CMD=${DOCKER_CMD:-"sudo docker"}

# Override the default CONFIG_ROOT path with one that is
# cluster-specific.
CONFIG_ROOT=${OPENSHIFT_CONFIG_ROOT:-/tmp/openshift-dind-cluster/${INSTANCE_PREFIX}}

DEPLOY_SSH=${OPENSHIFT_DEPLOY_SSH:-true}

DEPLOYED_CONFIG_ROOT="/config"

DEPLOYED_ROOT="/data"

SCRIPT_ROOT="${DEPLOYED_ROOT}/contrib/vagrant"

function check-selinux() {
  if [ "$(getenforce)" = "Enforcing" ]; then
    >&2 echo "Error: This script is not compatible with SELinux enforcing mode."
    exit 1
  fi
}

IMAGE_REGISTRY="${OPENSHIFT_TEST_IMAGE_REGISTRY:-}"
IMAGE_TAG="${OPENSHIFT_TEST_IMAGE_TAG:-}"
DIND_IMAGE="${IMAGE_REGISTRY}openshift/dind${IMAGE_TAG}"
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
    if [ -n "${IMAGE_REGISTRY}" ]; then
      # Failure to cache is assumed to not be worth failing the build.
      ${DOCKER_CMD} pull "${DIND_IMAGE}" || true
    fi
    build-image "${ORIGIN_ROOT}/images/dind" "${DIND_IMAGE}"
    if [ -n "${IMAGE_REGISTRY}" ]; then
      ${DOCKER_CMD} push "${DIND_IMAGE}" || true
    fi
  fi
}

function get-docker-ip() {
  local cid=$1

  ${DOCKER_CMD} inspect --format '{{ .NetworkSettings.IPAddress }}' "${cid}"
}

function docker-exec-script() {
    local cid=$1
    local cmd=$2

    ${DOCKER_CMD} exec -t "${cid}" ${BASH_CMD} ${cmd}
}

function start() {
  # docker-in-docker's use of volumes is not compatible with SELinux
  check-selinux

  echo "Configured network plugin: ${NETWORK_PLUGIN}"

  # TODO(marun) - perform these operations in a container for boot2docker compat
  echo "Ensuring compatible host configuration"
  sudo modprobe openvswitch
  sudo modprobe br_netfilter 2> /dev/null || true
  sudo sysctl -w net.bridge.bridge-nf-call-iptables=0 > /dev/null
  mkdir -p "${CONFIG_ROOT}"

  if [ "${SKIP_BUILD}" = "true" ]; then
    echo "WARNING: Skipping image build due to OPENSHIFT_SKIP_BUILD=true"
  else
    build-images
  fi

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
  node_ips=$(os::provision::join , ${node_ips[@]})

  ## Provision containers
  local args="${master_ip} ${NODE_COUNT} ${node_ips} ${INSTANCE_PREFIX} \
-n ${NETWORK_PLUGIN}"
  if [ "${SKIP_BUILD}" = "true" ]; then
      args="${args} -s"
  fi
  if [ "${SDN_NODE}" = "true" ]; then
      args="${args} -o"
  fi

  echo "Provisioning ${MASTER_NAME}"
  local cmd="${SCRIPT_ROOT}/provision-master.sh ${args} -c \
${DEPLOYED_CONFIG_ROOT}"
  docker-exec-script "${master_cid}" "${cmd}"

  if [ "${DEPLOY_SSH}" = "true" ]; then
    ${DOCKER_CMD} exec -t "${master_cid}" ssh-keygen -N '' -q -f /root/.ssh/id_rsa
    cmd="cat /root/.ssh/id_rsa.pub"
    local public_key=$(${DOCKER_CMD} exec -t "${master_cid}" ${cmd})
    cmd="cp /root/.ssh/id_rsa.pub /root/.ssh/authorized_keys"
    ${DOCKER_CMD} exec -t "${master_cid}" ${cmd}
    ${DOCKER_CMD} exec -t "${master_cid}" systemctl start sshd
  fi

  # Ensure that all users (e.g. outside the container) have read-write
  # access to the openshift configuration.  Security shouldn't be a
  # concern for dind since it should only be used for dev and test.
  local openshift_config_path="${CONFIG_ROOT}/openshift.local.config"
  find "${openshift_config_path}" -exec sudo chmod ga+rw {} \;
  find "${openshift_config_path}" -type d -exec sudo chmod ga+x {} \;

  for (( i=0; i < ${#node_cids[@]}; i++ )); do
    local node_index=$((i + 1))
    local cid="${node_cids[$i]}"
    local name="${NODE_NAMES[$i]}"
    echo "Provisioning ${name}"
    cmd="${SCRIPT_ROOT}/provision-node.sh ${args} -i ${node_index} -c \
${DEPLOYED_CONFIG_ROOT}"
    docker-exec-script "${cid}" "${cmd}"

    if [ "${DEPLOY_SSH}" = "true" ]; then
      ${DOCKER_CMD} exec -t "${cid}" mkdir -p /root/.ssh
      cmd="echo ${public_key} > /root/.ssh/authorized_keys"
      ${DOCKER_CMD} exec -t "${cid}" bash -c "${cmd}"
      ${DOCKER_CMD} exec -t "${cid}" systemctl start sshd
    fi
  done

  local rc_file="dind-${INSTANCE_PREFIX}.rc"
  local admin_config=$(os::provision::get-admin-config ${CONFIG_ROOT})
  echo "export KUBECONFIG=${admin_config}
export PATH=\$PATH:${ORIGIN_ROOT}/_output/local/bin/linux/amd64" > "${rc_file}"

  if [ "${KUBECONFIG:-}" != "${admin_config}" ]; then
    echo ""
    echo "Before invoking the openshift cli, make sure to source the
cluster's rc file to configure the bash environment:

  $ . ${rc_file}
  $ oc get nodes
"
  fi
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

  echo "Cleanup up configuration to avoid conflict with a future cluster"
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

# Build and deploy openshift binaries to an existing cluster
function redeploy() {
  local node_service="openshift-node"

  ${DOCKER_CMD} exec -t "${MASTER_NAME}" bash -c "\
. ${SCRIPT_ROOT}/provision-util.sh ; \
os::provision::build-origin ${DEPLOYED_ROOT} ${SKIP_BUILD}"

  echo "Stopping ${MASTER_NAME} service(s)"
  ${DOCKER_CMD} exec -t "${MASTER_NAME}" systemctl stop "${MASTER_NAME}"
  if [ "${SDN_NODE}" = "true" ]; then
    ${DOCKER_CMD} exec -t "${MASTER_NAME}" systemctl stop "${node_service}"
  fi
  echo "Updating ${MASTER_NAME} binaries"
  ${DOCKER_CMD} exec -t "${MASTER_NAME}" bash -c \
". ${SCRIPT_ROOT}/provision-util.sh ; \
os::provision::install-cmds ${DEPLOYED_ROOT}"
  echo "Starting ${MASTER_NAME} service(s)"
  ${DOCKER_CMD} exec -t "${MASTER_NAME}" systemctl start "${MASTER_NAME}"
  if [ "${SDN_NODE}" = "true" ]; then
    ${DOCKER_CMD} exec -t "${MASTER_NAME}" systemctl start "${node_service}"
  fi

  for node_name in "${NODE_NAMES[@]}"; do
    echo "Stopping ${node_name} service"
    ${DOCKER_CMD} exec -t "${node_name}" systemctl stop "${node_service}"
    echo "Updating ${node_name} binaries"
    ${DOCKER_CMD} exec -t "${node_name}" bash -c "\
. ${SCRIPT_ROOT}/provision-util.sh ; \
os::provision::install-cmds ${DEPLOYED_ROOT}"
    echo "Starting ${node_name} service"
    ${DOCKER_CMD} exec -t "${node_name}" systemctl start "${node_service}"
  done
}

function nodes-are-ready() {
    local node_count=$(${DOCKER_CMD} exec -t "${MASTER_NAME}" bash -c "\
KUBECONFIG=${DEPLOYED_CONFIG_ROOT}/openshift.local.config/master/admin.kubeconfig \
oc get nodes | grep Ready | wc -l")
    node_count=$(echo "${node_count}" | tr -d '\r')
    test "${node_count}" -ge "${NODE_COUNT}"
}

function wait-for-cluster() {
  local msg="nodes to register with the master"
  local condition="nodes-are-ready"
  os::provision::wait-for-condition "${msg}" "${condition}"
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
  redeploy)
    redeploy
    ;;
  wait-for-cluster)
    wait-for-cluster
    ;;
  build-images)
    BUILD_IMAGES=1
    build-images
    ;;
  config-host)
    os::provision::set-os-env "${ORIGIN_ROOT}" "${CONFIG_ROOT}"
    ;;
  *)
    echo "Usage: $0 {start|stop|restart|redeploy|wait-for-cluster|build-images|config-host}"
    exit 2
esac
