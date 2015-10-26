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

os::util::set-os-env() {
  local origin_root=$1
  local config_root=$2

  # Set up the KUBECONFIG environment variable for use by oc.
  #
  # Target .bashrc since docker exec doesn't invoke .bash_profile and
  # .bash_profile loads .bashrc anyway.
  local file_target=".bashrc"

  local vagrant_target="/home/vagrant/${file_target}"
  if [ -d $(dirname "${vagrant_target}") ]; then
    os::util::set-bash-env "${origin_root}" "${config_root}" \
"${vagrant_target}"
  fi
  os::util::set-bash-env "${origin_root}" "${config_root}" \
"/root/${file_target}"
}

os::util::set-bash-env() {
  local origin_root=$1
  local config_root=$2
  local target=$3

  local path="${config_root}/openshift.local.config/master/admin.kubeconfig"
  local config_line="export KUBECONFIG=${path}"
  if ! grep -q "${config_line}" "${target}" &> /dev/null; then
    echo "export KUBECONFIG=${path}" >> "${target}"
    echo "cd ${origin_root}" >> "${target}"
  fi
}

os::util::get-network-plugin() {
  local plugin=$1
  local dind_management_script=${2:-false}

  local subnet_plugin="redhat/openshift-ovs-subnet"
  local multitenant_plugin="redhat/openshift-ovs-multitenant"
  local default_plugin="${subnet_plugin}"

  if [ "${plugin}" != "${subnet_plugin}" ] && \
     [ "${plugin}" != "${multitenant_plugin}" ]; then
    # Disable output when being called from the dind management script
    # since it may be doing something other than launching a cluster.
    if [ "${dind_management_script}" = "false" ]; then
      if [ "${plugin}" != "" ]; then
        >&2 echo "Invalid network plugin: ${plugin}"
      fi
      >&2 echo "Using default network plugin: ${default_plugin}"
    fi
    plugin="${default_plugin}"
  fi
  echo "${plugin}"
}

os::util::install-sdn() {
  local deployed_root=$1

  # The subnet plugin is discovered via the kube network plugin path.
  local kube_osdn_path="/usr/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet"
  mkdir -p "${kube_osdn_path}"

  # Source scripts from an openshift-sdn repo if present to support
  # openshift-sdn development.
  local sdn_root="${deployed_root}/third-party/openshift-sdn"
  if [ -d "${sdn_root}" ]; then
    >&2 echo "Sourcing sdn scripts from ${sdn_root}"
    pushd "${sdn_root}/pkg/ovssubnet/controller" > /dev/null
      ln -rsf kube/bin/openshift-ovs-subnet "${kube_osdn_path}/"
      ln -rsf kube/bin/openshift-sdn-kube-subnet-setup.sh /usr/bin/

      ln -rsf multitenant/bin/openshift-ovs-multitenant /usr/bin/
      ln -rsf multitenant/bin/openshift-sdn-multitenant-setup.sh /usr/bin/
    popd > /dev/null
  else
    local osdn_base_path="${deployed_root}/Godeps/_workspace/src/github.com/openshift/openshift-sdn"
    local osdn_controller_path="${osdn_base_path}/pkg/ovssubnet/controller"
    pushd "${osdn_controller_path}" > /dev/null
      cp -f kube/bin/openshift-ovs-subnet "${kube_osdn_path}/"
      cp -f kube/bin/openshift-sdn-kube-subnet-setup.sh /usr/bin/

      # The multitenant plugin only needs to be in PATH because the
      # origin multitenant plugin knows how to discover it.
      cp -f multitenant/bin/openshift-ovs-multitenant /usr/bin/
      cp -f multitenant/bin/openshift-sdn-multitenant-setup.sh /usr/bin/
    popd > /dev/null
  fi

  # subnet and multitenant plugin setup writes docker network options
  # to /run/openshift-sdn/docker-network, make this file to be exported
  # as part of docker service start.
  local system_docker_path="/usr/lib/systemd/system/docker.service.d/"
  mkdir -p "${system_docker_path}"
  cat <<EOF > "${system_docker_path}/docker-sdn-ovs.conf"
[Service]
EnvironmentFile=-/run/openshift-sdn/docker-network
EOF

  systemctl enable openvswitch
  systemctl start openvswitch
}

os::util::base-provision() {
  os::util::fixup-net-udev

  os::util::setup-hosts-file "${MASTER_NAME}" "${MASTER_IP}" NODE_NAMES NODE_IPS

  os::util::install-pkgs
}

os::util::fixup-net-udev() {
  if [ "${FIXUP_NET_UDEV}" == "true" ]; then
    NETWORK_CONF_PATH=/etc/sysconfig/network-scripts/
    rm -f ${NETWORK_CONF_PATH}ifcfg-enp*
    if [[ -f "${NETWORK_CONF_PATH}ifcfg-eth1" ]]; then
      sed -i 's/^NM_CONTROLLED=no/#NM_CONTROLLED=no/' ${NETWORK_CONF_PATH}ifcfg-eth1
      if ! grep -q "NAME=" ${NETWORK_CONF_PATH}ifcfg-eth1; then
        echo "NAME=openshift" >> ${NETWORK_CONF_PATH}ifcfg-eth1
      fi
      nmcli con reload
      nmcli dev disconnect eth1
      nmcli con up "openshift"
    fi
  fi
}

os::util::install-pkgs() {
  # Only install packages if not deploying to a container.  A
  # container is expected to have installed packages as part of image
  # creation.
  if [ ! -f /.dockerinit ]; then
    yum update -y
    yum install -y docker-io git golang e2fsprogs hg net-tools bridge-utils which ethtool
  fi
}

os::util::start-os-service() {
  local unit_name=$1
  local description=$2
  local exec_start=$3
  local work_dir=${4:-${CONFIG_ROOT}/}

  # TODO(marun) Should the daemons be sharing a working directory?

  cat <<EOF > "/usr/lib/systemd/system/${unit_name}.service"
[Unit]
Description=${description}
Requires=network.target
After=docker.target network.target

[Service]
ExecStart=${exec_start}
WorkingDirectory=${work_dir}
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "${unit_name}.service"
systemctl start "${unit_name}.service"

}

os::util::start-node-service() {
  local node_name=$1

  # Copy over the certificates directory so that each node has a copy.
  cp -r "${CONFIG_ROOT}/openshift.local.config" /
  if [ -d /home/vagrant ]; then
    chown -R vagrant.vagrant /openshift.local.config
  fi

  cmd="/usr/bin/openshift start node --loglevel=${LOG_LEVEL} \
--config=/openshift.local.config/node-${node_name}/node-config.yaml"
  os::util::start-os-service "openshift-node" "OpenShift Node" "${cmd}" /
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

os::util::is-sdn-node-registered() {
  local master_cid=$1
  local node_name=$2

  ${DOCKER_CMD} exec -t "${master_cid}" bash -ci \
    "oc get nodes ${node_name} &> /dev/null"
}

os::util::disable-sdn-node() {
  local master_cid=$1
  local node_name=$2

  local sdn_msg="for sdn node to register with the master"
  local start_msg="Waiting ${sdn_msg}"
  local error_msg="[ERROR] Timeout waiting ${sdn_msg}"
  local condition="os::util::is-sdn-node-registered ${master_cid} ${node_name}"
  local timeout=30
  os::util::wait-for-condition "${start_msg}" "${error_msg}" "${condition}" \
    "${timeout}"

  echo "Disabling scheduling for the sdn node"
  # Disable scheduling outside of the master provision script to give
  # the node time to register itself to the master.
  ${DOCKER_CMD} exec -t "${master_cid}" bash -ci \
    "osadm manage-node ${node_name} --schedulable=false > /dev/null"
}
