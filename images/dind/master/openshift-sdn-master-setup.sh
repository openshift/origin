#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source /usr/local/bin/openshift-dind-lib.sh
source /data/dind-env

function ensure-token() {
  local token_file="${1}/openshift-sdn.token"
  local kube_config=$2

  /usr/local/bin/oc --config="${kube_config}" sa get-token openshift-sdn > ${token_file}
  [[ -s "${token_file}" ]]
}

# Create a ServiceAccount for the openshift-sdn node processes. The token
# isn't (yet) used by the master, but we generate it in the master to avoid
# synchronizing between nodes.
function openshift-sdn-setup() {
  local config_dir=$1
  local kube_config="${config_dir}/admin.kubeconfig"

  os::util::wait-for-apiserver "${kube_config}"

  # Create the service account for openshift-sdn
  if ! /usr/local/bin/oc --config="${kube_config}" get serviceaccount openshift-sdn >/dev/null 2>&1; then
    /usr/local/bin/oc --config="${kube_config}" create serviceaccount openshift-sdn
    /usr/local/bin/oc --config="${kube_config}" adm policy add-cluster-role-to-user cluster-admin -z openshift-sdn

    # rhbz#1383707: need to add openshift-sdn SA to anyuid SCC to allow pod annotation updates
    os::util::wait-for-anyuid "${kube_config}"
    /usr/local/bin/oc --config="${kube_config}" adm policy add-scc-to-user anyuid -z openshift-sdn
  fi

  os::util::wait-for-condition "kubernetes token" "ensure-token ${config_dir} ${kube_config}" "120"
}


if [[ "${OPENSHIFT_NETWORK_PLUGIN}" =~ ^"redhat/" ]]; then
  openshift-sdn-setup /data/openshift.local.config/master
fi
