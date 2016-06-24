#!/bin/bash

os::provision::install-sdn() {
  local default_target="/usr"

  local deployed_root=$1
  local target=${2:-${default_target}}

  if [ ! -d ${target} ]; then
    mkdir -p ${target}
  fi

  local osdn_plugin_path="${deployed_root}/pkg/sdn/plugin"
  mkdir -p "${target}/bin/"
  pushd "${osdn_plugin_path}" > /dev/null
    install bin/openshift-sdn-ovs "${target}/bin/"
    install bin/openshift-sdn-docker-setup.sh "${target}/bin/"
  popd > /dev/null

  # osdn plugin setup writes docker network options to
  # /run/openshift-sdn/docker-network, make this file to be exported
  # as part of docker service start.
  local system_docker_path="${target}/lib/systemd/system/docker.service.d/"
  mkdir -p "${system_docker_path}"
  cat <<EOF > "${system_docker_path}/docker-sdn-ovs.conf"
[Service]
EnvironmentFile=-/run/openshift-sdn/docker-network
EOF

  # Assume a non-default target is an indication of deploying in an
  # environment where openvswitch is managed in a separate container
  # (e.g. atomic host).
  if [[ "${target}" = "${default_target}" ]]; then
    systemctl enable openvswitch
    systemctl start openvswitch
  fi
}
