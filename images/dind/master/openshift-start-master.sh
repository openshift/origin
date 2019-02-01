#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# Should set OPENSHIFT_NETWORK_PLUGIN
source /data/dind-env
source /data/hack/lib/init.sh
source /data/hack/local-up-master/lib.sh

function start-kube-scheduler() {
    local scheduler_dir="${LOCALUP_CONFIG}/kube-scheduler"
    local scheduler_kubeconfig="${scheduler_dir}/kube-scheduler.kubeconfig"

    CERT_DIR=${LOCALUP_CONFIG}/kube-apiserver
    ROOT_CA_FILE=${CERT_DIR}/server-ca.crt
    if [ ! -f "${scheduler_kubeconfig}" ]; then
        mkdir -p "${scheduler_dir}"
        kube::util::create_client_certkey "${CONTROLPLANE_SUDO}" "${CERT_DIR}" 'client-ca' kube-scheduler system:kube-scheduler
        kube::util::write_client_kubeconfig "${CONTROLPLANE_SUDO}" "${CERT_DIR}" "${ROOT_CA_FILE}" "${API_HOST}" "${API_SECURE_PORT}" kube-scheduler
        cp ${LOCALUP_CONFIG}/kube-apiserver/kube-scheduler.kubeconfig ${scheduler_kubeconfig}
    fi

    KUBE_SCHEDULER_LOG=${LOG_DIR}/kube-scheduler.log
    hyperkube kube-scheduler \
      --v=${LOG_LEVEL} \
      --vmodule="${LOG_SPEC}" \
      --kubeconfig ${scheduler_kubeconfig} \
      --cert-dir="${CERT_DIR}" \
      --leader-elect=false >"${KUBE_SCHEDULER_LOG}" 2>&1 &
    KUBE_SCHEDULER_PID=$!

    os::log::debug "Waiting for kube-scheduler to come up"
    kube::util::wait_for_url "http://localhost:10251/healthz" "kube-scheduler: " 1 ${WAIT_FOR_URL_API_SERVER} ${MAX_TIME_FOR_URL_API_SERVER} \
        || { os::log::error "check kube-scheduler logs: ${KUBE_SCHEDULER_LOG}" ; exit 1 ; }
}

function ensure-master-config() {
  local config_path="/data/openshift.local.config"
  local master_path="${config_path}/master"
  local name="$(hostname)"

  local ip_addr1
  ip_addr1="$(ip addr | grep inet | grep eth0 | awk '{print $2}' | sed -e 's+/.*++')"

  local ip_addr2
  ip_addr2="$(ip addr | grep inet | (grep eth1 || true) | awk '{print $2}' | sed -e 's+/.*++')"

  local ip_addrs
  local serving_ip_addr
  if [[ -n "${ip_addr2}" ]]; then
    ip_addrs="${ip_addr1},${ip_addr2}"
    serving_ip_addr="${ip_addr2}"
  else
    ip_addrs="${ip_addr1}"
    serving_ip_addr="${ip_addr1}"
  fi
  export API_HOST_IP=${serving_ip_addr}
  export API_HOST=${serving_ip_addr}

  mkdir -p "${master_path}"
  export LOCALUP_ROOT="${config_path}"
  export LOCALUP_CONFIG="${master_path}"
  export HOSTNAME_OVERRIDE=${name}
  mkdir -p "${config_path}/logs"
  export LOG_DIR="${config_path}/logs"
  export ALLOWED_REGISTRIES='[{"domainName":"172.30.30.30:5000"},{"domainName":"myregistry.com"},{"domainName":"registry.centos.org"},{"domainName":"docker.io"},{"domainName":"gcr.io"},{"domainName":"quay.io"},{"domainName":"*.redhat.com"},{"domainName":"*.docker.io"},{"domainName":"registry.redhat.io"}]'

  # Make sure the podman database gets initialized with our non-default graph driver
  podman ps

  (flock 200;
   os::util::environment::setup_all_server_vars
   os::util::ensure_tmpfs "${ETCD_DATA_DIR}"
   localup::init_master
   podman pull registry.svc.ci.openshift.org/openshift/origin-release:v4.0
   start-kube-scheduler
  ) 200>"${config_path}"/.openshift-ca.lock

  # ensure the configuration can be used outside of the container
  chmod -R ga+rX "${master_path}"
  chmod ga+w "${master_path}/admin.kubeconfig"
}

ensure-master-config

while true; do sleep 1; localup::healthcheck; done
