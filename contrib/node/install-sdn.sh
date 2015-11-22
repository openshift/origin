#!/bin/bash

os::provision::install-sdn() {
  local default_target="/usr"

  local deployed_root=$1
  local target=${2:-${default_target}}

  if [ ! -d ${target} ]; then
    mkdir -p ${target}
  fi

  local osdn_base_path="${deployed_root}/Godeps/_workspace/src/github.com/openshift/openshift-sdn"
  local osdn_controller_path="${osdn_base_path}/pkg/ovssubnet/controller"
  local kube_osdn_path="${target}/libexec/kubernetes/kubelet-plugins/net/exec/redhat~openshift-ovs-subnet"
  mkdir -p "${kube_osdn_path}"
  mkdir -p "${target}/bin/"

  pushd "${osdn_controller_path}" > /dev/null
    # The subnet plugin is discovered via the kube network plugin path.
    cp -f kube/bin/openshift-ovs-subnet "${kube_osdn_path}/"
    cp -f kube/bin/openshift-sdn-kube-subnet-setup.sh "${target}/bin/"

    # The multitenant plugin only needs to be in PATH because the
    # origin multitenant plugin knows how to discover it.
    cp -f multitenant/bin/openshift-ovs-multitenant "${target}/bin/"
    cp -f multitenant/bin/openshift-sdn-multitenant-setup.sh "${target}/bin/"
  popd > /dev/null

  # subnet and multitenant plugin setup writes docker network options
  # to /run/openshift-sdn/docker-network, make this file to be exported
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
