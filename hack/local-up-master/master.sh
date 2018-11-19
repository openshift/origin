#!/usr/bin/env bash

set -e

source "$(dirname "${BASH_SOURCE}")/../lib/init.sh"
source "$(dirname "${BASH_SOURCE}")/../local-up-master/lib.sh"

trap "clusterup::cleanup" EXIT

os::cleanup::tmpdir
os::util::environment::setup_all_server_vars
os::util::ensure_tmpfs "${ETCD_DATA_DIR}"

echo "Logging to ${LOG_DIR}..."

os::log::system::start

LOCALUP_ROOT=${LOCALUP_ROOT:-$(pwd)}
LOCALUP_CONFIG=${LOCALUP_ROOT}/openshift.local.masterup

localup::init_master

echo
echo "Cluster is available, use the following kubeconfig to interact with it"
echo "export KUBECONFIG=${LOCALUP_CONFIG}/admin.kubeconfig"
echo "Press ctrl+C to finish"

while true; do sleep 1; localup::healthcheck; done
