#!/bin/bash

# WARNING: The script modifies the host that docker is running on.  It
# attempts to load the overlay and openvswitch modules. If this modification
# is undesirable consider running docker in a VM.
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
# This script has been tested on Fedora 24, but should work on any
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
# (openshift.local.*) in /tmp/openshift-dind-cluster/openshift.  It's
# possible to run multiple dind clusters simultaneously by overriding
# the instance prefix.  The following command would ensure
# configuration was stored at /tmp/openshift-dind/cluster/my-cluster:
#
#    OPENSHIFT_CLUSTER_ID=my-cluster hack/dind-cluster.sh [command]
#
# Suggested Workflow
# ------------------
#
# When making changes to the deployment of a dind cluster or making
# breaking golang changes, the 'restart' command will ensure that an
# existing cluster is cleaned up before deploying a new cluster.
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

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
source "${OS_ROOT}/images/dind/node/openshift-dind-lib.sh"

function start() {
  local origin_root=$1
  local config_root=$2
  local deployed_config_root=$3
  local cluster_id=$4
  local network_plugin=$5
  local wait_for_cluster=$6
  local node_count=$7

  # docker-in-docker's use of volumes is not compatible with SELinux
  check-selinux

  echo "Starting dind cluster '${cluster_id}' with plugin '${network_plugin}'"

  # Ensuring compatible host configuration
  #
  # Running in a container ensures that the docker host will be affected even
  # if docker is running remotely.  The openshift/dind-node image was chosen
  # due to its having sysctl installed.
  ${DOCKER_CMD} run --privileged --net=host --rm -v /lib/modules:/lib/modules \
                openshift/dind-node bash -e -c \
                '/usr/sbin/modprobe openvswitch;
                /usr/sbin/modprobe overlay 2> /dev/null || true;'

  # Initialize the cluster config path
  mkdir -p "${config_root}"
  echo "OPENSHIFT_NETWORK_PLUGIN=${network_plugin}" > "${config_root}/network-plugin"
  copy-runtime "${origin_root}" "${config_root}/"

  local volumes="-v ${config_root}:${deployed_config_root}"
  local run_cmd="${DOCKER_CMD} run -dt ${volumes}  --privileged"

  # Create containers
  ${run_cmd} --name="${MASTER_NAME}" --hostname="${MASTER_NAME}" "${MASTER_IMAGE}" > /dev/null
  for name in "${NODE_NAMES[@]}"; do
    ${run_cmd} --name="${name}" --hostname="${name}" "${NODE_IMAGE}" > /dev/null
  done

  local rc_file="dind-${cluster_id}.rc"
  local admin_config
  admin_config="$(get-admin-config "${CONFIG_ROOT}")"
  local bin_path
  bin_path="$(os::build::get-bin-output-path "${OS_ROOT}")"
  cat >"${rc_file}" <<EOF
export KUBECONFIG=${admin_config}
export PATH=\$PATH:${bin_path}
EOF

  if [[ -n "${wait_for_cluster}" ]]; then
    wait-for-cluster "${config_root}" "${node_count}"
  fi

  if [[ "${KUBECONFIG:-}" != "${admin_config}"  ||
          ":${PATH}:" != *":${bin_path}:"* ]]; then
    echo ""
    echo "Before invoking the openshift cli, make sure to source the
cluster's rc file to configure the bash environment:

  $ . ${rc_file}
  $ oc get nodes
"
  fi
}

function stop() {
  local config_root=$1
  local cluster_id=$2

  echo "Stopping dind cluster '${cluster_id}'"

  local master_cid
  master_cid="$(${DOCKER_CMD} ps -qa --filter "name=${MASTER_NAME}")"
  if [[ "${master_cid}" ]]; then
    ${DOCKER_CMD} rm -f "${master_cid}" > /dev/null
  fi

  local node_cids
  node_cids="$(${DOCKER_CMD} ps -qa --filter "name=${NODE_PREFIX}")"
  if [[ "${node_cids}" ]]; then
    node_cids=(${node_cids//\n/ })
    for cid in "${node_cids[@]}"; do
      ${DOCKER_CMD} rm -f "${cid}" > /dev/null
    done
  fi

  # Cleaning up configuration to avoid conflict with a future cluster
  # The container will have created configuration as root
  sudo rm -rf "${config_root}"/openshift.local.etcd
  sudo rm -rf "${config_root}"/openshift.local.config

  # Cleanup orphaned volumes
  #
  # See: https://github.com/jpetazzo/dind#important-warning-about-disk-usage
  #
  for volume in $( ${DOCKER_CMD} volume ls -qf dangling=true ); do
    ${DOCKER_CMD} volume rm "${volume}" > /dev/null
  done
}

function check-selinux() {
  if [[ "$(getenforce)" = "Enforcing" ]]; then
    >&2 echo "Error: This script is not compatible with SELinux enforcing mode."
    exit 1
  fi
}

function get-network-plugin() {
  local plugin=$1

  local subnet_plugin="redhat/openshift-ovs-subnet"
  local multitenant_plugin="redhat/openshift-ovs-multitenant"
  local default_plugin="${multitenant_plugin}"

  if [[ "${plugin}" != "${subnet_plugin}" &&
          "${plugin}" != "${multitenant_plugin}" &&
          "${plugin}" != "cni" ]]; then
    if [[ -n "${plugin}" ]]; then
      >&2 echo "Invalid network plugin: ${plugin}"
    fi
    plugin="${default_plugin}"
  fi
  echo "${plugin}"
}

function get-docker-ip() {
  local cid=$1

  ${DOCKER_CMD} inspect --format '{{ .NetworkSettings.IPAddress }}' "${cid}"
}

function get-admin-config() {
  local config_root=$1

  echo "${config_root}/openshift.local.config/master/admin.kubeconfig"
}

function copy-runtime() {
  local origin_root=$1
  local target=$2

  cp "$(os::build::find-binary openshift)" "${target}"
  cp "$(os::build::find-binary host-local)" "${target}"
  cp "$(os::build::find-binary loopback)" "${target}"
  cp "$(os::build::find-binary sdn-cni-plugin)" "${target}/openshift-sdn"
  local osdn_plugin_path="${origin_root}/pkg/sdn/plugin"
  cp "${osdn_plugin_path}/bin/openshift-sdn-ovs" "${target}"
  cp "${osdn_plugin_path}/sdn-cni-plugin/80-openshift-sdn.conf" "${target}"
}

function wait-for-cluster() {
  local config_root=$1
  local expected_node_count=$2

  # Increment the node count to ensure that the sdn node also reports readiness
  (( expected_node_count++ ))

  local kubeconfig
  kubeconfig="$(get-admin-config "${config_root}")"
  local oc
  oc="$(os::build::find-binary oc)"

  local msg="${expected_node_count} nodes to report readiness"
  local condition="nodes-are-ready ${kubeconfig} ${oc} ${expected_node_count}"
  local timeout=120
  os::util::wait-for-condition "${msg}" "${condition}" "${timeout}"
}

function nodes-are-ready() {
  local kubeconfig=$1
  local oc=$2
  local expected_node_count=$3

  # TODO - do not count any node whose name matches the master node e.g. 'node-master'
  read -d '' template <<'EOF'
{{range $item := .items}}
  {{range .status.conditions}}
    {{if eq .type "Ready"}}
      {{if eq .status "True"}}
        {{printf "%s\\n" $item.metadata.name}}
      {{end}}
    {{end}}
  {{end}}
{{end}}
EOF
  # Remove formatting before use
  template="$(echo "${template}" | tr -d '\n' | sed -e 's/} \+/}/g')"
  local count
  count="$("${oc}" --config="${kubeconfig}" get nodes \
                   --template "${template}" 2> /dev/null | \
                   wc -l)"
  test "${count}" -ge "${expected_node_count}"
}

function build-images() {
  local origin_root=$1

  echo "Building container images"
  build-image "${origin_root}/images/dind/" "${BASE_IMAGE}"
  build-image "${origin_root}/images/dind/node" "${NODE_IMAGE}"
  build-image "${origin_root}/images/dind/master" "${MASTER_IMAGE}"
}

function build-image() {
  local build_root=$1
  local image_name=$2

  pushd "${build_root}" > /dev/null
    ${DOCKER_CMD} build -t "${image_name}" .
  popd > /dev/null
}

DOCKER_CMD=${DOCKER_CMD:-"sudo docker"}

CLUSTER_ID="${OPENSHIFT_CLUSTER_ID:-openshift}"

TMPDIR="${TMPDIR:-"/tmp"}"
CONFIG_ROOT="${OPENSHIFT_CONFIG_ROOT:-${TMPDIR}/openshift-dind-cluster/${CLUSTER_ID}}"
DEPLOYED_CONFIG_ROOT="/data"

MASTER_NAME="${CLUSTER_ID}-master"
NODE_PREFIX="${CLUSTER_ID}-node-"
NODE_COUNT=2
NODE_NAMES=()
for (( i=1; i<=NODE_COUNT; i++ )); do
  NODE_NAMES+=( "${NODE_PREFIX}${i}" )
done

BASE_IMAGE="openshift/dind"
NODE_IMAGE="openshift/dind-node"
MASTER_IMAGE="openshift/dind-master"

case "${1:-""}" in
  start)
    BUILD=
    BUILD_IMAGES=
    WAIT_FOR_CLUSTER=1
    NETWORK_PLUGIN=
    REMOVE_EXISTING_CLUSTER=
    OPTIND=2
    while getopts ":bin:rs" opt; do
      case $opt in
        b)
          BUILD=1
          ;;
        i)
          BUILD_IMAGES=1
          ;;
        n)
          NETWORK_PLUGIN="${OPTARG}"
          ;;
        r)
          REMOVE_EXISTING_CLUSTER=1
          ;;
        s)
          WAIT_FOR_CLUSTER=
          ;;
        \?)
          echo "Invalid option: -${OPTARG}" >&2
          exit 1
          ;;
        :)
          echo "Option -${OPTARG} requires an argument." >&2
          exit 1
          ;;
      esac
    done

    if [[ -n "${REMOVE_EXISTING_CLUSTER}" ]]; then
      stop "${CONFIG_ROOT}" "${CLUSTER_ID}"
    fi

    # Build origin if requested or required
    if [[ -n "${BUILD}" || -z "$(os::build::find-binary oc)" ]]; then
      "${OS_ROOT}/hack/build-go.sh"
    fi

    # Build images if requested or required
    if [[ -n "${BUILD_IMAGES}" ||
            -z "$(${DOCKER_CMD} images -q ${MASTER_IMAGE})" ]]; then
      build-images "${OS_ROOT}"
    fi

    NETWORK_PLUGIN="$(get-network-plugin "${NETWORK_PLUGIN}")"
    start "${OS_ROOT}" "${CONFIG_ROOT}" "${DEPLOYED_CONFIG_ROOT}" \
          "${CLUSTER_ID}" "${NETWORK_PLUGIN}" "${WAIT_FOR_CLUSTER}" \
          "${NODE_COUNT}" "${NODE_PREFIX}"
    ;;
  stop)
    stop "${CONFIG_ROOT}" "${CLUSTER_ID}"
    ;;
  wait-for-cluster)
    wait-for-cluster "${CONFIG_ROOT}" "${NODE_COUNT}"
    ;;
  build-images)
    build-images "${OS_ROOT}"
    ;;
  *)
    >&2 echo "Usage: $0 {start|stop|wait-for-cluster|build-images}

start accepts the following arguments:

 -n [net plugin]   the name of the network plugin to deploy

 -b                build origin before starting the cluster

 -i                build container images before starting the cluster

 -r                remove an existing cluster

 -s                skip waiting for nodes to become ready
"
    exit 2
esac
