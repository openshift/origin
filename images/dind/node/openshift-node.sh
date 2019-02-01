#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

source /usr/local/bin/openshift-dind-lib.sh
# Should set OPENSHIFT_NETWORK_PLUGIN
source /data/dind-env
source /data/hack/lib/init.sh
source /data/hack/local-up-master/lib.sh

function run-hyperkube() {
  local config_path="/data/openshift.local.config"
  local host
  host="$(hostname)"
  if os::util::is-master; then
    host="${host}-node"
  fi
  local master_config_path="${config_path}/master"
  local master_config_file="${master_config_path}/admin.kubeconfig"

  # Wait for the master to generate its config
  local condition="test -f ${master_config_file}"
  os::util::wait-for-condition "admin config" "${condition}"

  local master_host
  master_host="$(grep server "${master_config_file}" | grep -v localhost | awk '{print $2}')"

  local ip_addr1
  ip_addr1="$(ip addr | grep inet | grep eth0 | awk '{print $2}' | sed -e 's+/.*++')"

  local ip_addr2
  ip_addr2="$(ip addr | grep inet | (grep eth1 || true) | awk '{print $2}' | sed -e 's+/.*++')"

  local ip_addrs
  if [[ -n "${ip_addr2}" ]]; then
    ip_addrs="${ip_addr1},${ip_addr2}"
  else
    ip_addrs="${ip_addr1}"
  fi

  mkdir -p /etc/kubernetes/manifests
  cp ${master_config_path}/admin.kubeconfig /etc/kubernetes/kubeconfig

  # Make sure the podman database gets initialized with our non-default graph driver
  podman ps

  # Hold a lock on the shared volume to ensure cert generation is
  # performed serially.  Cert generation is not compatible with
  # concurrent execution since the file passed to --signer-serial
  # needs to be incremented by each invocation.
#  (flock 200;
#    LOCALUP_CONFIG=${master_config_path}
#    CERT_DIR=${LOCALUP_CONFIG}/kube-apiserver
#    HOSTNAME_OVERRIDE=${host}
#    kube::util::create_client_certkey "" "${CERT_DIR}" 'client-ca' kubelet system:node:${HOSTNAME_OVERRIDE} system:nodes
#  ) 200>"${config_path}"/.openshift-ca.lock

    /usr/local/bin/hyperkube kubelet \
      --container-runtime=remote \
      --container-runtime-endpoint=/var/run/crio/crio.sock \
      --image-service-endpoint=/var/run/crio/crio.sock \
      --runtime-request-timeout=10m \
      --pod-manifest-path=/etc/kubernetes/manifests \
      --allow-privileged=true \
      --minimum-container-ttl-duration=6m0s \
      --cluster-domain=cluster.local \
      --cgroup-driver=systemd \
      --cgroups-per-qos=false \
      --serialize-image-pulls=false \
      --v=4 \
      --fail-swap-on=false \
      --enforce-node-allocatable="" \
      --eviction-hard="" \
      --eviction-soft="" \
      --client-ca-file=${master_config_path}/kube-apiserver/client-ca.crt \
      --tls-cert-file=${master_config_path}/kube-apiserver/client-kubelet.crt \
      --tls-private-key-file=${master_config_path}/kube-apiserver/client-kubelet.key \
      --cluster-dns=172.30.0.1 \
      --hostname-override=${host} \
      --kubeconfig=${master_config_path}/admin.kubeconfig \
      --network-plugin=cni \
      --pod-infra-container-image=openshift/origin-pod:${OPENSHIFT_IMAGE_VERSION}\
      --root-dir=/var/lib/origin/openshift.local.volumes
}

run-hyperkube
