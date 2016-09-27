#!/bin/bash

os::provision::install-sdn() {
  local deployed_root=$1
  local target=${2:-}
  local target_usrdir="${target}/usr"
  local target_bindir="${target_usrdir}/bin"
  local target_etcdir="${target}/etc"
  local target_cnidir="${target}/opt/cni/bin"

  mkdir -p ${target_usrdir}
  mkdir -p ${target_bindir}
  mkdir -p ${target_etcdir}
  mkdir -p ${target_cnidir}

  local osdn_plugin_path="${deployed_root}/pkg/sdn/plugin"
  pushd "${osdn_plugin_path}" > /dev/null
    install sdn-cni-plugin/openshift-sdn-ovs "${target_bindir}"
  popd > /dev/null

  # openshift-sdn places a CNI config file here
  mkdir -p "${target_etcdir}/cni/net.d"

  install "${OS_OUTPUT_BINPATH}/$(os::build::host_platform)/sdn-cni-plugin" "${target_cnidir}/openshift-sdn"
  install "${OS_OUTPUT_BINPATH}/$(os::build::host_platform)/host-local" "${target_cnidir}/host-local"

  # Assume an empty/default target is an indication of deploying in an
  # environment where openvswitch should be started by us
  if [[ -z "${target}" ]]; then
    systemctl enable openvswitch
    systemctl start openvswitch
  fi
}
