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
  cluster_cidr=$(python -c "import yaml; stream = file('${master_config}', 'r'); y = yaml.load(stream); print y['networkConfig']['clusterNetworks'][0]['cidr']")
  service_cidr=$(python -c "import yaml; stream = file('${master_config}', 'r'); y = yaml.load(stream); print y['networkConfig']['serviceNetworkCIDR']")
  apiserver=$(awk '/server:/ { print $2; exit }' ${kube_config})
  ovn_master_ip=$(echo -n ${apiserver} | cut -d "/" -f 3 | cut -d ":" -f 1)

  ovn-nbctl set-connection ptcp:6641
  ovn-sbctl set-connection ptcp:6642

  if [[ -n "${OPENSHIFT_OVN_HYBRID_OVERLAY}" ]]; then
    HYBRID_OVERLAY_ARGS="--enable-hybrid-overlay --hybrid-overlay-cluster-subnets=11.128.0.0/16/24 --no-hostsubnet-nodes=beta.kubernetes.io/os=windows"
  fi

  echo "Enabling and start ovn-kubernetes master services"
  /usr/local/bin/ovnkube \
	--loglevel=5 \
	--k8s-apiserver "${apiserver}" \
	--k8s-cacert "${config_dir}/ca.crt" \
	--k8s-token "${token}" \
	--cluster-subnet "${cluster_cidr}" \
	--k8s-service-cidr "${service_cidr}" \
	--nb-address "tcp://${ovn_master_ip}:6641" \
	--sb-address "tcp://${ovn_master_ip}:6642" \
	--init-master `hostname` \
	${HYBRID_OVERLAY_ARGS:-}
}

if [[ -n "${OPENSHIFT_OVN_KUBERNETES}" ]]; then
  ovn-kubernetes-master /data/openshift.local.config/master
fi
