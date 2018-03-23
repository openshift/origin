#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source /usr/local/bin/openshift-dind-lib.sh
source /data/dind-env

function is-northd-running() {
  local northd_ip=$1

  ovn-nbctl --timeout=2 "--db=tcp:${northd_ip}:6641" ls-list
}

function have-token() {
  local master_dir=$1

  [[ -s "${master_dir}/ovn.token" ]]
}

function ovn-kubernetes-node() {
  local config_dir=$1
  local master_dir=$2
  local kube_config="${config_dir}/node.kubeconfig"

  os::util::wait-for-condition "kubernetes token" "have-token ${master_dir}" "120"

  token=$(cat ${master_dir}/ovn.token)

  cat >"/etc/openvswitch/ovn_k8s.conf" <<EOF
[kubernetes]
cacert=${config_dir}/ca.crt
EOF

  local host
  host="$(hostname)"
  if os::util::is-master; then
    host="${host}-node"
  fi

  local node_config="${config_dir}/node-config.yaml"
  local master_config="${master_dir}/master-config.yaml"
  cluster_cidr=$(python -c "import yaml; stream = file('${master_config}', 'r'); y = yaml.load(stream); print y['networkConfig']['clusterNetworkCIDR']")
  apiserver=$(grep server ${kube_config} | cut -f 6 -d' ')
  ovn_master_ip=$(echo -n ${apiserver} | cut -d "/" -f 3 | cut -d ":" -f 1)

  # Ensure GENEVE's UDP port isn't firewalled
  /usr/share/openvswitch/scripts/ovs-ctl --protocol=udp --dport=6081 enable-protocol

  os::util::wait-for-condition "ovn-northd" "is-northd-running ${ovn_master_ip}" "120"

  echo "Enabling and start ovn-kubernetes node services"
  /usr/local/bin/ovnkube \
	--k8s-apiserver "${apiserver}" \
	--k8s-cacert "${config_dir}/ca.crt" \
	--k8s-token "${token}" \
	--cluster-subnet "${cluster_cidr}" \
	--nb-address "tcp://${ovn_master_ip}:6641" \
	--sb-address "tcp://${ovn_master_ip}:6642" \
	--init-node ${host} \
	--init-gateways
}

if [[ -n "${OPENSHIFT_OVN_KUBERNETES}" ]]; then
  ovn-kubernetes-node /var/lib/origin/openshift.local.config/node /data/openshift.local.config/master
fi
