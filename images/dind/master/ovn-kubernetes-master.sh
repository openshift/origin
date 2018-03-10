#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source /usr/local/bin/openshift-dind-lib.sh
source /data/dind-env

function ovn-kubernetes-master() {
  local config_dir=$1
  local kube_config="${config_dir}/admin.kubeconfig"

  token=$(cat ${config_dir}/ovn.token)

  local master_config="${config_dir}/master-config.yaml"
  cluster_cidr=$(python -c "import yaml; stream = file('${master_config}', 'r'); y = yaml.load(stream); print y['networkConfig']['clusterNetworkCIDR']")
  apiserver=$(oc --config="${kube_config}" config view -o custom-columns=server:clusters[0].cluster.server | grep http)
  ovn_master_ip=$(echo -n ${apiserver} | cut -d "/" -f 3 | cut -d ":" -f 1)

  echo "Enabling and start ovn-kubernetes master services"
  /usr/local/bin/ovnkube \
	--k8s-apiserver "${apiserver}" \
	--k8s-cacert "${config_dir}/ca.crt" \
	--k8s-token "${token}" \
	--cluster-subnet "${cluster_cidr}" \
	--nb-address "tcp://${ovn_master_ip}:6641" \
	--sb-address "tcp://${ovn_master_ip}:6642" \
	--init-master `hostname` \
	--net-controller
}

if [[ -n "${OPENSHIFT_OVN_KUBERNETES}" ]]; then
  ovn-kubernetes-master /data/openshift.local.config/master
fi
