#!/bin/sh
set -euo pipefail
trap 'kill $(jobs -p); exit 0' TERM INT

master_config_path=$(pwd)/_output/local/server/master
node_config_path=$(pwd)/_output/local/server/node
volume_dir=${VOLUME_DIR:-$(pwd)/_output/local/server/node/volumes}
mkdir -p ${master_config_path} ${node_config_path} ${volume_dir}

host=$( openshift start master --print-ip )
openshift start master \
  --etcd-dir=${master_config_path}/etcd \
  --write-config=${master_config_path}
oc adm create-node-config \
  --node-dir="${node_config_path}" \
  --node="${host}" \
  --master="https://${host}:8443" \
  --dns-ip="${host}" \
  --dns-bind-address="${host}:53" \
  --hostnames="${host}" \
  --network-plugin="" \
  --volume-dir="${volume_dir}" \
  --node-client-certificate-authority="${master_config_path}/ca.crt" \
  --certificate-authority="${master_config_path}/ca.crt" \
  --signer-cert="${master_config_path}/ca.crt" \
  --signer-key="${master_config_path}/ca.key" \
  --signer-serial="${master_config_path}/ca.serial.txt"

flags=$( openshift-node-config --config=${node_config_path}/node-config.yaml )

( eval "hyperkube kubelet ${flags} &> $(pwd)/_output/local/server/kubelet.log" ) &
( while true; do
    if openshift-sdn --kubeconfig=${master_config_path}/admin.kubeconfig --enable=dns,proxy --config=${node_config_path}/node-config.yaml &> $(pwd)/_output/local/server/network.log; then
      break
    fi
    sleep 2
  done 
) &
( openshift start master --config=${master_config_path}/master-config.yaml &> $(pwd)/_output/local/server/master.log ) &

echo "Master at ${host}, KUBECONFIG=${master_config_path}/admin.kubeconfig"

wait