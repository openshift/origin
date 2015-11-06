#!/bin/bash

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
