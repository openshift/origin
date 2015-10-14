#!/bin/bash

os::util::join() {
  local IFS="$1"

  shift
  echo "$*"
}

os::util::install-cmds() {
  local deployed_root=$1

  cp ${deployed_root}/_output/local/bin/linux/amd64/{openshift,oc,osadm} /usr/bin
}

os::util::add-to-hosts-file() {
  local ip=$1
  local name=$2
  local force=${3:-0}

  if ! grep -q "${ip}" /etc/hosts || [ "${force}" = "1" ]; then
    local entry="${ip}\t${name}"
    echo -e "Adding '${entry}' to hosts file"
    echo -e "${entry}" >> /etc/hosts
  fi
}

os::util::setup-hosts-file() {
  local master_name=$1
  local master_ip=$2
  local -n node_names=$3
  local -n node_ips=$4

  # Setup hosts file to support ping by hostname to master
  os::util::add-to-hosts-file "${master_ip}" "${master_name}"

  # Setup hosts file to support ping by hostname to each node in the cluster
  for (( i=0; i < ${#node_names[@]}; i++ )); do
    os::util::add-to-hosts-file "${node_ips[$i]}" "${node_names[$i]}"
  done
}

os::util::init-certs() {
  local config_root=$1
  local network_plugin=$2
  local master_name=$3
  local master_ip=$4
  local -n node_names=$5
  local -n node_ips=$6

  local server_config_dir=${config_root}/openshift.local.config
  local volumes_dir="/var/lib/openshift.local.volumes"
  local cert_dir="${server_config_dir}/master"

  echo "Generating certs"

  pushd "${config_root}" > /dev/null

  # Master certs
  /usr/bin/openshift admin ca create-master-certs \
    --overwrite=false \
    --cert-dir="${cert_dir}" \
    --master="https://${master_ip}:8443" \
    --hostnames="${master_ip},${master_name}"

  # Certs for nodes
  for (( i=0; i < ${#node_names[@]}; i++ )); do
    local name=${node_names[$i]}
    local ip=${node_ips[$i]}
    /usr/bin/openshift admin create-node-config \
      --node-dir="${server_config_dir}/node-${name}" \
      --node="${name}" \
      --hostnames="${name},${ip}" \
      --master="https://${master_ip}:8443" \
      --network-plugin="${network_plugin}" \
      --node-client-certificate-authority="${cert_dir}/ca.crt" \
      --certificate-authority="${cert_dir}/ca.crt" \
      --signer-cert="${cert_dir}/ca.crt" \
      --signer-key="${cert_dir}/ca.key" \
      --signer-serial="${cert_dir}/ca.serial.txt" \
      --volume-dir="${volumes_dir}"
  done

  popd > /dev/null
}

# Set up the KUBECONFIG environment variable for use by oc
os::util::set-oc-env() {
  local config_root=$1
  local target=$2

  if [ "${config_root}" = "/" ]; then
    config_root=""
  fi

  local path="${config_root}/openshift.local.config/master/admin.kubeconfig"
  local config_line="export KUBECONFIG=${path}"
  if ! grep -q "${config_line}" "${target}" &> /dev/null; then
    echo "export KUBECONFIG=${path}" >> "${target}"
  fi
}

os::util::get-network-plugin() {
  local plugin=$1

  local subnet_plugin="redhat/openshift-ovs-subnet"
  local multitenant_plugin="redhat/openshift-ovs-multitenant"
  local default_plugin="${subnet_plugin}"

  if [ "${plugin}" != "${subnet_plugin}" ] && \
     [ "${plugin}" != "${multitenant_plugin}" ]; then
    if [ "${plugin}" != "" ]; then
        >&2 echo "Invalid network plugin: ${plugin}"
    fi
    >&2 echo "Using default network plugin: ${default_plugin}"
    plugin="${default_plugin}"
  fi
  echo "${plugin}"
}

os::util::install-sdn() {
  local deployed_root=$1
  local target=$2
  target=${target:-/usr}
  if [ ! -d ${target} ]; then
    mkdir -p ${target}
  fi
  # Source scripts from an openshift-sdn repo if present to support
  # openshift-sdn development.
  local sdn_root="${deployed_root}/third-party/openshift-sdn"
  if [ -d "${sdn_root}" ]; then
    pushd "${sdn_root}" > /dev/null
    # TODO: Enable these commands once we have a separate binary for openshift-sdn
    # make
    # make "install-dev"
    popd > /dev/null
  else
    local osdn_base_path="${deployed_root}/Godeps/_workspace/src/github.com/openshift/openshift-sdn"
    local osdn_controller_path="${osdn_base_path}/pkg/ovssubnet/controller"
    pushd "${osdn_controller_path}" > /dev/null
      # The subnet plugin is discovered via the kube network plugin path.
      local kube_osdn_path="${target}/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet"
      mkdir -p "${kube_osdn_path}"
      mkdir -p "${target}/bin/"
      cp -f kube/bin/openshift-ovs-subnet "${kube_osdn_path}/"
      cp -f kube/bin/openshift-sdn-kube-subnet-setup.sh "${target}/bin/"

      # The multitenant plugin only needs to be in PATH because the
      # origin multitenant plugin knows how to discover it.
      cp -f multitenant/bin/openshift-ovs-multitenant "${target}/bin/"
      cp -f multitenant/bin/openshift-sdn-multitenant-setup.sh "${target}/bin/"

      # subnet and multitenant plugin setup writes docker network options
      # to /run/openshift-sdn/docker-network, make this file to be exported
      # as part of docker service start.
      local system_docker_path="${target}/lib/systemd/system/docker.service.d/"
      mkdir -p "${system_docker_path}"
      cat <<EOF > "${system_docker_path}/docker-sdn-ovs.conf"
[Service]
EnvironmentFile=-/run/openshift-sdn/docker-network
EOF
    popd > /dev/null
  fi

}

os::util::wait-for-condition() {
  local start_msg=$1
  local error_msg=$2
  # condition should be a string that can be eval'd.  When eval'd, it
  # should not output anything to stderr or stdout.
  local condition=$3
  local timeout=${4:-30}
  local sleep_interval=${5:-1}

  local counter=0
  while ! $(${condition}); do
    if [ "${counter}" = "0" ]; then
      echo "${start_msg}"
    fi

    if [[ "${counter}" -lt "${timeout}" ]]; then
      counter=$((counter + 1))
      echo -n '.'
      sleep 1
    else
      echo -e "\n${error_msg}"
      return 1
    fi
  done

  if [ "${counter}" != "0" ]; then
    echo -e '\nDone'
  fi
}
