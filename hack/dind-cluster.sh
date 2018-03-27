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
# breaking golang changes, the -r argument to the start command will
# ensure that an existing cluster is cleaned up before deploying a new
# cluster.
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
  local ovn_root=$2
  local config_root=$3
  local deployed_config_root=$4
  local cluster_id=$5
  local network_plugin=$6
  local container_runtime=$7
  local wait_for_cluster=$8
  local node_count=$9
  local additional_args=${10}

  # docker-in-docker's use of volumes is not compatible with SELinux
  check-selinux

  runtime_endpoint=
  if [[ "${container_runtime}" = "dockershim" ]]; then
    # dockershim is default and doesn't need an endpoint path
    runtime_endpoint=
  elif [[ "${container_runtime}" = "crio" ]]; then
    runtime_endpoint="unix:///var/run/crio/crio.sock"
  else
    >&2 echo "Invalid container runtime: ${container_runtime}"
    exit 1
  fi

  echo "Starting dind cluster '${cluster_id}' with plugin '${network_plugin}' and runtime '${container_runtime}'"

  # Error if a cluster is already configured
  check-no-containers "start"

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
  echo "OPENSHIFT_NETWORK_PLUGIN=${network_plugin}" > "${config_root}/dind-env"
  echo "OPENSHIFT_ADDITIONAL_ARGS='${additional_args}'" >> "${config_root}/dind-env"
  copy-runtime "${origin_root}" "${config_root}/"

  echo "OPENSHIFT_CONTAINER_RUNTIME=${container_runtime}" >> "${config_root}/dind-env"
  echo "OPENSHIFT_REMOTE_RUNTIME_ENDPOINT=${runtime_endpoint}" >> "${config_root}/dind-env"

  ovn_kubernetes=
  if [[ -d "${ovn_root}" ]]; then
    copy-ovn-runtime "${ovn_root}" "${config_root}/"
    ovn_kubernetes=1
  fi
  echo "OPENSHIFT_OVN_KUBERNETES=${ovn_kubernetes}" >> "${config_root}/dind-env"

  # Create containers
  start-container "${config_root}" "${deployed_config_root}" "${MASTER_IMAGE}" "${MASTER_NAME}"
  for name in "${NODE_NAMES[@]}"; do
    start-container "${config_root}" "${deployed_config_root}" "${NODE_IMAGE}" "${name}"
  done

  if [[ -n "${ADDITIONAL_NETWORK_INTERFACE}" ]]; then
    add-network-interface-to-nodes "${config_root}"
    update-master-config
    update-node-config
  fi

  local rc_file="dind-${cluster_id}.rc"
  local admin_config
  admin_config="$(get-admin-config "${CONFIG_ROOT}")"
  local bin_path
  bin_path="$(os::build::get-bin-output-path "${OS_ROOT}")"
  cat >"${rc_file}" <<EOF
export KUBECONFIG="${admin_config}"
export PATH="\$PATH:${bin_path}"

export OPENSHIFT_CLUSTER_ID="${cluster_id}"
export OPENSHIFT_CONFIG_ROOT="${config_root}"

for file in "${origin_root}/contrib/completions/bash"/* ; do
    source "\${file}"
done
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

function add-network-interface-to-nodes () {
  config_root=$1

  # Create new bridge
  sudo brctl addbr "${ADDITIONAL_BRIDGE_NAME}"
  # Assign IPAM to the bridge
  sudo ifconfig "${ADDITIONAL_BRIDGE_NAME}" "172.${ADDITIONAL_NETWORK_NUM}.0.1/16"
  echo "OPENSHIFT_ADDITIONAL_BRIDGE_NAME=${ADDITIONAL_BRIDGE_NAME}" >> "${config_root}/dind-env"

  local netns_path="/var/run/netns"
  sudo mkdir -p "${netns_path}"

  local num=3
  for pid in $( ${DOCKER_CMD} ps -q --filter "name=${MASTER_NAME}|${NODE_PREFIX}" | xargs ${DOCKER_CMD} inspect --format '{{.State.Pid}}' ); do
    # Link container network namespace so that 'ip netns' can recognize
    sudo ln -s /proc/"${pid}"/ns/net "${netns_path}/ns-${pid}"
    # Create veth pair
    sudo ip link add veth1-ns-"${pid}" type veth peer name veth2-ns-"${pid}"
    # Move one end of the veth pair inside the container
    sudo ip link set veth1-ns-"${pid}" netns ns-"${pid}"
    # Rename interface name inside the container
    sudo ip netns exec ns-"${pid}" ip link set veth1-ns-"${pid}" name "${ADDITIONAL_IFACE_NAME}"
    # Assign address to the added interface in the container
    sudo ip netns exec ns-"${pid}" ifconfig "${ADDITIONAL_IFACE_NAME}" "172.${ADDITIONAL_NETWORK_NUM}.0.${num}/24"
    # Bring up the link connected to the container
    sudo ip netns exec ns-"${pid}" ip link set "${ADDITIONAL_IFACE_NAME}" up
    # Move other end of the veth pair to the bridge
    sudo brctl addif "${ADDITIONAL_BRIDGE_NAME}" veth2-ns-"${pid}"
    # Bring up the link connected to the bridge
    sudo ip link set dev veth2-ns-"${pid}" up

    (( num += 1 ))
  done
}

function update-master-config() {
  local config_path="/data/openshift.local.config"
  local master_config_path="${config_path}/master"
  local master_config_file="${master_config_path}/master-config.yaml"

  # Remove master config file to trigger master config regeneration
  #
  # openshift-generate-master-config.sh script repopulates master config
  # with certs for both eth0 and eth1 IP addrs.
  #
  # openshift-master service executes openshift-generate-master-config.sh
  # as pre start hook.
  rm -f "${master_config_file}"
}

function update-node-config() {
  local config_path="/data/openshift.local.config"
  local host="$(hostname)"
  local node_config_path="${config_path}/node-${host}"
  local node_config_file="${node_config_path}/node-config.yaml"

  # Remove node config file to trigger node config regeneration
  #
  # openshift-generate-node-config.sh script repopulates node config
  # with certs for both eth0 and eth1 IP addrs.
  #
  # openshift-node service executes openshift-generate-node-config.sh
  # as pre start hook.
  rm -f "${node_config_file}"
}

function add-node () {
  local config_root=$1
  local deployed_config_root=$2
  local cluster_id=$3
  local wait_for_cluster=$4

  echo "Adding node to dind cluster '${cluster_id}'"

  # Error if a cluster is not already configured
  check-containers "add-node"

  # Find the first free number
  local first_free=1
  for cid in $( ${DOCKER_CMD} ps -a --filter "name=${NODE_PREFIX}" --format "{{.Names}}" | sort -V ); do
    if [[ "${cid}" != "${NODE_PREFIX}${first_free}" ]]; then
      break
    fi
    (( first_free += 1 ))
  done
  local node_name="${NODE_PREFIX}${first_free}"

  start-container "${config_root}" "${deployed_config_root}" "${NODE_IMAGE}" "${node_name}"
  echo
  echo "Added node '${node_name}'"

  if [[ -n "${wait_for_cluster}" ]]; then
    wait-for-cluster "${config_root}" "$(count-nodes)"
  fi
}

function start-container() {
  local config_root=$1
  local deployed_config_root=$2
  local image=$3
  local name=$4

  local volumes run_cmd
  volumes=""

  ${DOCKER_CMD} run -dt ${volumes} -v "${config_root}:${deployed_config_root}" --privileged \
                --name="${name}" --hostname="${name}" "${image}" > /dev/null
}

function delete-node () {
  local config_root=$1
  local cluster_id=$2
  local node_name=$3

  echo "Removing node '${node_name}' from dind cluster '${cluster_id}'"
  sudo echo -n

  # Error if a cluster is not already configured
  check-containers "delete-node"
  check-containers "delete-node" "-f name=${node_name}" "No node named '${node_name}'"

  # Remove it from docker
  ${DOCKER_CMD} rm -f "${node_name}"
  clean-orphaned-storage

  # Remove the stale config
  sudo rm -rf "${config_root}"/openshift.local.config/node-"${node_name}"

  # Delete it from openshift
  local kubeconfig oc
  kubeconfig="$(get-admin-config "${config_root}")"
  oc="$(os::util::find::built_binary oc) --config=${kubeconfig}"

  ${oc} delete node "${node_name}"
}

function stop() {
  local config_root=$1
  local cluster_id=$2

  echo "Stopping dind cluster '${cluster_id}'"
  sudo echo -n

  # Delete additional bridge if present
  additional_bridge_name=$(cat ${config_root}/dind-env | grep OPENSHIFT_ADDITIONAL_BRIDGE_NAME | cut -d'=' -f2 || true)
  if [[ -n "${additional_bridge_name}" ]]; then
      sudo ip link set "${additional_bridge_name}" down 2> /dev/null || "true"
      sudo brctl delbr "${additional_bridge_name}" 2> /dev/null || "true"
  fi

  # Delete the containers
  for cid in $( ${DOCKER_CMD} ps -qa --filter "name=${MASTER_NAME}|${NODE_PREFIX}" ); do
    ${DOCKER_CMD} rm -f "${cid}" > /dev/null
  done

  # Cleaning up configuration to avoid conflict with a future cluster
  # The container will have created configuration as root
  sudo rm -rf "${config_root}"/openshift.local.etcd
  sudo rm -rf "${config_root}"/openshift.local.config

  clean-orphaned-storage
}

function copy-image() {
  local cluster_id=$1
  local image=$2

  echo "Installing image '${image}' on all nodes of dind cluster '${cluster_id}'"

  # Error if a cluster is not configured
  check-containers "copy-image"
  check-no-containers "copy-image" "-f status=exited" "Paused parts"

  # Make a temp file with a descriptor attached
  tmpfile=$(mktemp /tmp/docker-image.XXXXXX)
  exec 3>"$tmpfile"
  rm "$tmpfile"

  ${DOCKER_CMD} save "${image}" >&3

  for cid in $( ${DOCKER_CMD} ps -qa --filter "name=${NODE_PREFIX}" ); do
    cat /dev/fd/3 | ${DOCKER_CMD} exec -i "${cid}" docker load &
  done

  wait
}

function clean-orphaned-storage() {
  # Cleanup orphaned volumes
  #
  # See: https://github.com/jpetazzo/dind#important-warning-about-disk-usage
  #
  for volume in $( ${DOCKER_CMD} volume ls -qf dangling=true ); do
    ${DOCKER_CMD} volume rm "${volume}" > /dev/null
  done
}

function pause() {
  local cluster_id=$1

  echo "Pausing dind cluster '${cluster_id}'"

  # Error if a cluster is not configured
  check-containers "pause"

  for cid in $( ${DOCKER_CMD} ps -qa --filter "name=${MASTER_NAME}|${NODE_PREFIX}" ); do
    ${DOCKER_CMD} stop "${cid}" > /dev/null &
  done

  wait
}

function resume() {
  local config_root=$1
  local cluster_id=$2
  local wait_for_cluster=$3

  echo "Resuming dind cluster '${cluster_id}'"
  sudo echo -n

  # Error if there are no containers configured or if something isn't paused
  check-containers "resume"
  check-containers "resume" "-f status=exited" "No paused parts"

  # Remove the old config and the serving certificates in case the IP changed... they will get regenerated
  local local_config
  local_config="${config_root}/openshift.local.config"
  sudo rm -f "${local_config}"/*/master.server.*
  sudo rm -f "${local_config}"/*/etcd.server.*
  sudo rm -f "${local_config}"/*/*.kubeconfig
  sudo rm -f "${local_config}"/*/master-config.yaml

  # Remove the node configs so that the nodes re-make their configuration
  sudo rm -f "${config_root}"/openshift.local.config/node-openshift-*/node-config.yaml

  # Do restarts in case they were already running (because we just nuked the config)
  echo "  Starting master"
  for cid in $( ${DOCKER_CMD} ps -qa --filter "name=${MASTER_NAME}" ); do
    ${DOCKER_CMD} restart "${cid}" > /dev/null
  done

  echo "  Starting nodes"
  for cid in $( ${DOCKER_CMD} ps -qa --filter "name=${NODE_PREFIX}" ); do
    ${DOCKER_CMD} restart "${cid}" > /dev/null
  done

  if [[ -n "${wait_for_cluster}" ]]; then
    wait-for-cluster "${config_root}" "$(count-nodes)"
  fi
}

function run-on-nodes() {
  local filter=$1
  shift
  local command=("$@")

  for cid in $( ${DOCKER_CMD} ps -qa --filter "name=${filter}" ); do
    ${DOCKER_CMD} exec "${cid}" "${command[@]}"
  done
}

function cluster-ps() {
  ${DOCKER_CMD} ps -a --filter "name=${MASTER_NAME}|${NODE_PREFIX}"
}

function count-masters() {
    local filter=${1:-}
    count-containers "${MASTER_NAME}" "${filter}"
}

function count-nodes() {
    local filter=${1:-}
    count-containers "${NODE_PREFIX}" "${filter}"
}

function count-containers() {
  local name=$1
  local filter=${2:-}

  local count=0
  for cid in $( ${DOCKER_CMD} ps -qa --filter "name=${name}" $filter); do
    (( count += 1 ))
  done

  echo "$count"
}

function check-no-containers {
  local operation=$1
  local filter=${2:-}
  local message="${3:-Existing cluster parts}"

  local existing_nodes existing_master
  existing_nodes=$(count-nodes "${filter}")
  existing_master=$(count-masters "${filter}")
  if (( existing_nodes > 0 || existing_master > 0 )); then
    echo
    echo "ERROR: Can't ${operation}.  ${message} (${existing_master} existing master or ${existing_nodes} existing nodes)"
    exit 1
  fi
}

function check-containers {
  local operation=$1
  local filter=${2:-}
  local message="${3:-No existing cluster parts}"

  local existing_nodes existing_master
  existing_nodes=$(count-nodes "${filter}")
  existing_master=$(count-masters "${filter}")
  if (( existing_nodes == 0 && existing_master == 0 )); then
    echo
    echo "ERROR: Can't ${operation}.  ${message} (${existing_master} existing master or ${existing_nodes} existing nodes)"
    exit 1
  fi
}

function refresh() {
  local origin_root=$1
  local config_root=$2
  local cluster_id=$3
  local wait_for_cluster=$4

  echo "Refreshing dind cluster '${cluster_id}'"

  # Error if a cluster is not configured or if there are paused parts
  check-containers "refresh"
  check-no-containers "refresh" "-f status=exited" "Paused parts"

  # Stop the master and node openshift processes
  echo "  Stopping master and node processes in cluster '${cluster_id}'..."
  run-on-nodes "${MASTER_NAME}"                systemctl stop openshift-master.service
  run-on-nodes "${MASTER_NAME}|${NODE_PREFIX}" systemctl stop openshift-node.service

  # Wipe the state
  run-on-nodes "${NODE_PREFIX}" ovs-ofctl del-flows -O OpenFlow13 br0

  # Copy over the new openshift binaries
  echo "  Copying new runtime to cluster '${cluster_id}'..."
  copy-runtime "${origin_root}" "${config_root}/"

  # Restart the master and node openshift processes
  echo "  Restarting master and node processes in cluster '${cluster_id}'..."
  run-on-nodes "${MASTER_NAME}"                systemctl start openshift-master.service
  run-on-nodes "${MASTER_NAME}|${NODE_PREFIX}" systemctl start openshift-node.service

  if [[ -n "${wait_for_cluster}" ]]; then
    wait-for-cluster "${config_root}" "$(count-nodes)"
  fi
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
  local networkpolicy_plugin="redhat/openshift-ovs-networkpolicy"
  local ovn_plugin="ovn"
  local default_plugin="${multitenant_plugin}"

  if [[ "${plugin}" = "subnet" || "${plugin}" = "${subnet_plugin}" ]]; then
    echo "${subnet_plugin}"
  elif [[ "${plugin}" = "multitenant" || "${plugin}" = "${multitenant_plugin}" ]]; then
    echo "${multitenant_plugin}"
  elif [[ "${plugin}" = "networkpolicy" || "${plugin}" = "${networkpolicy_plugin}" ]]; then
    echo "${networkpolicy_plugin}"
  elif [[ "${plugin}" = "ovn" ]]; then
    echo "${ovn_plugin}"
  elif [[ "${plugin}" = "cni" ]]; then
    echo "cni"
  elif [[ "${plugin}" = "none" ]]; then
    echo ""
  elif [[ -n "${plugin}" ]]; then
    >&2 echo "Invalid network plugin: ${plugin}"
    exit 1
  else
    echo "${default_plugin}"
  fi
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

  cp "$(os::util::find::built_binary openshift)" "${target}"
  cp "$(os::util::find::built_binary oc)" "${target}"
  cp "$(os::util::find::built_binary host-local)" "${target}"
  cp "$(os::util::find::built_binary loopback)" "${target}"
  cp "$(os::util::find::built_binary sdn-cni-plugin)" "${target}/openshift-sdn"
}

function copy-ovn-runtime() {
  local ovn_root=$1
  local target=$2

  local ovn_go_controller_built_binaries_path="${ovn_root}/go-controller/_output/go/bin"
  cp "${ovn_go_controller_built_binaries_path}/ovnkube" "${target}"
  cp "${ovn_go_controller_built_binaries_path}/ovn-kube-util" "${target}"
  cp "${ovn_go_controller_built_binaries_path}/ovn-k8s-overlay" "${target}"
  cp "${ovn_go_controller_built_binaries_path}/ovn-k8s-cni-overlay" "${target}"
}

function wait-for-cluster() {
  local config_root=$1
  local expected_node_count=$2

  # Make sure there is a cluster configured
  check-containers "wait-for-cluster"

  # Increment the node count to ensure that the sdn node on the master also reports readiness
  (( expected_node_count += 1 ))

  local kubeconfig oc
  kubeconfig="$(get-admin-config "${config_root}")"
  oc="$(os::util::find::built_binary oc)"

  # wait for healthz to report ok before trying to get nodes
  os::util::wait-for-condition "ok" "${oc} get --config=${kubeconfig} --raw=/healthz" "120"

  local msg condition timeout
  msg="${expected_node_count} nodes to report readiness"
  condition="nodes-are-ready ${kubeconfig} ${oc} ${expected_node_count}"
  timeout=120
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

  os::build::image "${image_name}" "${build_root}"
}

function os::build::get-bin-output-path() {
  local os_root="${1:-}"

  if [[ -n "${os_root}" ]]; then
    os_root="${os_root}/"
  fi
  echo ${os_root}_output/local/bin/$(os::build::host_platform)
}

## Start of the main program

DEFAULT_DOCKER_CMD="sudo docker"
if [[ -w "/var/run/docker.sock" ]]; then
  DEFAULT_DOCKER_CMD="docker"

  # Since docker is a shell script we do not want to pass our restrictions to it
  # This would be stripped by sudo, but we have to do it manually otherwise
  export -n SHELLOPTS
else
  # Make sure that they don't do half the work if sudo fails later by getting it primed now
  sudo echo -n
fi
DOCKER_CMD="${DOCKER_CMD:-$DEFAULT_DOCKER_CMD}"

CLUSTER_ID="${OPENSHIFT_CLUSTER_ID:-openshift}"

TMPDIR="${TMPDIR:-"/tmp"}"
CONFIG_BASE="${OPENSHIFT_CONFIG_BASE:-${TMPDIR}/openshift-dind-cluster}"
CONFIG_ROOT="${OPENSHIFT_CONFIG_ROOT:-${CONFIG_BASE}/${CLUSTER_ID}}"
DEPLOYED_CONFIG_ROOT="/data"

MASTER_NAME="${CLUSTER_ID}-master"
NODE_PREFIX="${CLUSTER_ID}-node-"
NODE_COUNT=2
NODE_NAMES=()

ADDITIONAL_BRIDGE_NAME="${ADDITIONAL_BRIDGE_NAME:-os-addbr-1}"
ADDITIONAL_NETWORK_NUM=18
ADDITIONAL_IFACE_NAME="${ADDITIONAL_IFACE_NAME:-eth1}"

BASE_IMAGE="openshift/dind"
NODE_IMAGE="openshift/dind-node"
MASTER_IMAGE="openshift/dind-master"
ADDITIONAL_ARGS=""

OVN_ROOT="${OVN_ROOT:-}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-dockershim}"

case "${1:-""}" in
  start)
    BUILD=
    BUILD_IMAGES=
    WAIT_FOR_CLUSTER=1
    NETWORK_PLUGIN=
    REMOVE_EXISTING_CLUSTER=
    ADDITIONAL_NETWORK_INTERFACE=
    OPTIND=2
    while getopts ":abc:in:rsN:" opt; do
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
        N)
          NODE_COUNT="${OPTARG}"
          ;;
        r)
          REMOVE_EXISTING_CLUSTER=1
          ;;
        s)
          WAIT_FOR_CLUSTER=
          ;;
        c)
          CONTAINER_RUNTIME="${OPTARG}"
          ;;
        a)
          ADDITIONAL_NETWORK_INTERFACE=1
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

   if [[ ${@:OPTIND-1:1} = "--" ]]; then
      ADDITIONAL_ARGS=${@:OPTIND}
   fi

   for (( i=1; i<=NODE_COUNT; i++ )); do
      NODE_NAMES+=( "${NODE_PREFIX}${i}" )
   done

    if [[ -n "${REMOVE_EXISTING_CLUSTER}" ]]; then
      stop "${CONFIG_ROOT}" "${CLUSTER_ID}"
    fi

    # Build origin if requested or required
    if [[ -n "${BUILD}" ]] || ! os::util::find::built_binary 'oc' >/dev/null 2>&1; then
      "${OS_ROOT}/hack/build-go.sh"
    fi

    # Build images if requested or required
    if [[ -n "${BUILD_IMAGES}" ||
            -z "$(${DOCKER_CMD} images -q ${MASTER_IMAGE})" ]]; then
      build-images "${OS_ROOT}"
    fi

    NETWORK_PLUGIN="$(get-network-plugin "${NETWORK_PLUGIN}")"

    # OVN requires CNI network plugin and OVN_ROOT to be set
    if [[ "${NETWORK_PLUGIN}" = "ovn" ]]; then
      NETWORK_PLUGIN="cni"
      if [[ -z "${OVN_ROOT}" ]]; then
        echo "OVN network plugin requires OVN_ROOT set to ovn-kubernetes checkout"
        exit 1
      fi
    elif [[ -n "${OVN_ROOT}" ]]; then
      OVN_ROOT=
    fi

    start "${OS_ROOT}" "${OVN_ROOT}" "${CONFIG_ROOT}" "${DEPLOYED_CONFIG_ROOT}" \
          "${CLUSTER_ID}" "${NETWORK_PLUGIN}" "${CONTAINER_RUNTIME}" \
          "${WAIT_FOR_CLUSTER}" "${NODE_COUNT}" "${ADDITIONAL_ARGS}"
    ;;
  add-node)
    WAIT_FOR_CLUSTER=1
    OPTIND=2
    while getopts ":bis" opt; do
      case $opt in
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
    add-node "${CONFIG_ROOT}" "${DEPLOYED_CONFIG_ROOT}" "${CLUSTER_ID}" "${WAIT_FOR_CLUSTER}"
    ;;
  delete-node)
    NODE_NUM=$2
    NODE_NAME="${NODE_PREFIX}${NODE_NUM}"
    delete-node "${CONFIG_ROOT}" "${CLUSTER_ID}" "${NODE_NAME}"
    ;;
  stop)
    stop "${CONFIG_ROOT}" "${CLUSTER_ID}"
    ;;
  refresh)
    BUILD=
    BUILD_IMAGES=
    WAIT_FOR_CLUSTER=1
    OPTIND=2
    while getopts ":bis" opt; do
      case $opt in
        b)
          BUILD=1
          ;;
        i)
          BUILD_IMAGES=1
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

    # Build origin if requested or required
    if [[ -n "${BUILD}" ]] || ! os::util::find::built_binary 'oc' >/dev/null 2>&1; then
      "${OS_ROOT}/hack/build-go.sh"
    fi

    # Build images if requested or required
    if [[ -n "${BUILD_IMAGES}" ||
            -z "$(${DOCKER_CMD} images -q ${MASTER_IMAGE})" ]]; then
      build-images "${OS_ROOT}"
    fi

    refresh "${OS_ROOT}" "${CONFIG_ROOT}" "${CLUSTER_ID}" "${WAIT_FOR_CLUSTER}"
    ;;
  pause)
    pause "${CLUSTER_ID}"
    ;;
  resume)
    WAIT_FOR_CLUSTER=1
    OPTIND=2
    while getopts ":s" opt; do
      case $opt in
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

    resume "${CONFIG_ROOT}" "${CLUSTER_ID}" "${WAIT_FOR_CLUSTER}"
    ;;
  copy-image)
    IMAGE_NAME=$2
    copy-image "${CLUSTER_ID}" "${IMAGE_NAME}"
    ;;
  wait-for-cluster)
    wait-for-cluster "${CONFIG_ROOT}" "${NODE_COUNT}"
    ;;
  build-images)
    build-images "${OS_ROOT}"
    ;;
  ps)
    cluster-ps
    ;;
  *)
    >&2 echo "Usage: $0 {start|stop|refresh|add-node|delete-node|ps|pause|resume|copy-image|wait-for-cluster|build-images} [options]

Commands:
- start: Starts the containers in an openshift docker-in-docker environment
- stop: Destroys the docker containers for the docker-in-docker environment
- add-node: Adds a node to the cluster
- delete-node <node-num>: Deletes the given node from the cluster
- refresh: Refreshes the openshift binaries in the containers and reloads the processes
- ps: List all of the docker containers that make up the cluster
- pause: Stops running containers, but leaves the state around
- resume: Restarts paused containers
- copy-image: Copies an image from the outer docker into all node dockers
- wait-for-cluster: Waits for a cluster to come online
- build-images: Builds the docker-in-docker images themselves


start accepts the following options:

 -n [net plugin]   the name of the network plugin to deploy (or "none" for none)
 -N                number of nodes in the cluster
 -b                build origin before starting the cluster
 -c [runtime name] use the specified container runtime instead of dockershim (eg, "crio")
 -i                build container images before starting the cluster
 -r                remove an existing cluster
 -a                add additional network interface to all nodes in the cluster
 -s                skip waiting for nodes to become ready

Any of the arguments that would be used in creating openshift master can be passed
as is to the script after '--' ex: setting host subnet to 3
./dind-cluster.sh start -- --host-subnet-length=3


refresh accepts the following options:
 -b                build origin before starting the cluster
 -i                build container images before starting the cluster
 -s                skip waiting for nodes to become ready

add-node and resume accept the following option:
 -s                skip waiting for nodes to become ready

The following environment variables are honored:
 - DOCKER_CMD: The docker command used.  Default: 'sudo docker'
 - OPENSHIFT_CLUSTER_ID: The name of the cluster (so multiple can be run). Default: 'openshift'
 - OPENSHIFT_CONFIG_BASE: Where the cluster configs are written, move somewhere persistent if you
      want to pause and resume across reboots.  Default: '${TMPDIR}/openshift-dind-cluster'
 - OPENSHIFT_CONFIG_ROOT: Where this specific cluster config is written.
      Default: '\${OPENSHIFT_CONFIG_BASE}/\${OPENSHIFT_CLUSTER_ID}'
"
    exit 2
esac
