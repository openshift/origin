#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source /data/dind-env
source /usr/local/bin/openshift-dind-lib.sh

function start-cluster-network-operator() {
  local config_path="/data/openshift.local.config"
  local master_config_path="${config_path}/master"
  local kubeconfig="${master_config_path}/admin.kubeconfig"

  # Wait for the master to generate its config
  local condition="test -f ${kubeconfig}"
  os::util::wait-for-condition "admin config" "${condition}"

  # Wait for the master to generate its config
  condition="/usr/local/bin/oc --config=${kubeconfig} get node openshift-master-node"
  os::util::wait-for-condition "master node" "${condition}"
  /usr/local/bin/oc --config="${kubeconfig}" label --overwrite node openshift-master-node node-role.kubernetes.io/master=""

  if ! oc --config="${kubeconfig}" get crd networks.config.openshift.io; then
    /usr/local/bin/oc --config="${kubeconfig}" create -f "/var/lib/networks.config.openshift.io-crd.yaml"
  fi

  local cno_path="${config_path}/cluster-network-operator"
  if ! oc --config="${kubeconfig}" get namespace openshift-network-operator; then
    /usr/local/bin/oc --config="${kubeconfig}" create -f "${cno_path}/manifests/0000_07_cluster-network-operator_00_namespace.yaml"
  fi
  if ! oc --config="${kubeconfig}" get crd networkconfigs.networkoperator.openshift.io; then
    /usr/local/bin/oc --config="${kubeconfig}" create -f "${cno_path}/manifests/0000_07_cluster-network-operator_01_crd.yaml" || true
  fi
  if ! oc --config="${kubeconfig}" get clusterrolebinding default-account-cluster-network-operator; then
    /usr/local/bin/oc --config="${kubeconfig}" create -f "${cno_path}/manifests/0000_07_cluster-network-operator_02_rbac.yaml" || true
  fi
  if ! oc --config="${kubeconfig}" get network.config.openshift.io cluster; then
    /usr/local/bin/oc --config="${kubeconfig}" create -f "${cno_path}/cluster-network-config.yaml" || true
  fi

  export NODE_IMAGE="docker.io/openshift/origin-node:${OPENSHIFT_IMAGE_VERSION}"
  export HYPERSHIFT_IMAGE="docker.io/openshift/origin-hypershift:${OPENSHIFT_IMAGE_VERSION}"
  export MULTUS_IMAGE="quay.io/openshift/origin-multus-cni:${OPENSHIFT_IMAGE_VERSION}"
  export CNI_PLUGINS_SUPPORTED_IMAGE="quay.io/openshift/origin-container-networking-plugins-supported:${OPENSHIFT_IMAGE_VERSION}"
  export CNI_PLUGINS_UNSUPPORTED_IMAGE="quay.io/openshift/origin-container-networking-plugins-unsupported:${OPENSHIFT_IMAGE_VERSION}"
  pushd "${cno_path}"
    POD_NAME=LOCAL ${cno_path}/bin/cluster-network-operator -v 6 --url-only-kubeconfig="${kubeconfig}" --kubeconfig="${kubeconfig}"
  popd
}

start-cluster-network-operator
