# Functions for setting up CT personalities in Trillian integration tests
# Requires github.com/google/trillian/integration/functions.sh

declare -a CT_SERVER_PIDS
CT_SERVERS=
CT_CFG=

# ct_prep_test prepares a set of running processes for a CT test.
# Parameters:
#   - number of log servers to run
#   - number of log signers to run
#   - number of CT personality instances to run
# Populates:
#  - CT_SERVERS         : list of HTTP addresses (comma separated)
#  - CT_METRICS_SERVERS : list of HTTP addresses (comma separated) serving metrics
#  - CT_SERVER_PIDS     : bash array of CT HTTP server pids
# in addition to the variables populated by log_prep_test.
# If etcd and Prometheus are configured, it also populates:
#  - ETCDISCOVER_PID : pid of etcd service watcher
#  - PROMETHEUS_PID  : pid of local Prometheus server
ct_prep_test() {
  # Default to one of everything.
  local rpc_server_count=${1:-1}
  local log_signer_count=${2:-1}
  local http_server_count=${3:-1}

  echo "Launching core Trillian log components"
  log_prep_test "${rpc_server_count}" "${log_signer_count}"

  echo "Building CT personality code"
  go build ${GOFLAGS} github.com/google/certificate-transparency-go/trillian/ctfe/ct_server

  echo "Provisioning logs for CT"
  ct_provision "${RPC_SERVER_1}"

  echo "Launching CT personalities"
  for ((i=0; i < http_server_count; i++)); do
    local port=$(pick_unused_port)
    CT_SERVERS="${CT_SERVERS},localhost:${port}"
    local metrics_port=$(pick_unused_port ${port})
    CT_METRICS_SERVERS="${CT_METRICS_SERVERS},localhost:${metrics_port}"

    echo "Starting CT HTTP server on localhost:${port}, metrics on localhost:${metrics_port}"
    ./ct_server ${ETCD_OPTS} --log_config="${CT_CFG}" --log_rpc_server="${RPC_SERVERS}" --http_endpoint="localhost:${port}" --metrics_endpoint="localhost:${metrics_port}" &
    pid=$!
    CT_SERVER_PIDS+=(${pid})
    wait_for_server_startup ${port}
  done
  CT_SERVERS="${CT_SERVERS:1}"
  CT_METRICS_SERVERS="${CT_METRICS_SERVERS:1}"

  if [[ ! -z "${ETCD_OPTS}" ]]; then
    echo "Registered HTTP endpoints"
    ETCDCTL_API=3 etcdctl get trillian-ctfe-http/ --prefix
    ETCDCTL_API=3 etcdctl get trillian-ctfe-metrics-http/ --prefix
  fi

  if [[ -x "${PROMETHEUS_DIR}/prometheus" ]]; then
    if [[ ! -z "${ETCD_OPTS}" ]]; then
        echo "Building etcdiscover"
        go build github.com/google/trillian/monitoring/prometheus/etcdiscover
        echo "Launching etcd service monitor"
        ./etcdiscover ${ETCD_OPTS} --etcd_services=trillian-ctfe-metrics-http,trillian-logserver-http,trillian-logsigner-http -target=./trillian.json --logtostderr &
        ETCDISCOVER_PID=$!
        echo "Launching Prometheus (default location localhost:9090)"
        ${PROMETHEUS_DIR}/prometheus --config.file=${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/integration/prometheus.yml \
                           --web.console.templates=${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/integration/consoles \
                           --web.console.libraries=${GOPATH}/src/github.com/google/certificate-transparency-go/third_party/prometheus/console_libs &
        PROMETHEUS_PID=$!
    fi
  fi
}

# ct_provision generates a CT configuration file and provisions the trees for it.
# Parameters:
#   - location of admin server instance
# Populates:
#   - CT_CFG : configuration file for CT personality
ct_provision() {
  local admin_server="$1"

  # Build config file with absolute paths
  CT_CFG=$(mktemp ${TMPDIR}/ct-XXXXXX)

  sed "s!@TESTDATA@!${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/testdata!" ${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/integration/ct_integration_test.cfg > "${CT_CFG}"

  echo 'Building createtree'
  go build ${GOFLAGS} github.com/google/trillian/cmd/createtree/

  num_logs=$(grep -c '@TREE_ID@' "${CT_CFG}")
  for i in $(seq ${num_logs}); do
    # TODO(daviddrysdale): Consider using distinct keys for each log
    tree_id=$(./createtree \
      --admin_server="${admin_server}" \
      --private_key_format=PrivateKey \
      --pem_key_path=${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/testdata/log-rpc-server.privkey.pem \
      --pem_key_password=towel \
      --signature_algorithm=ECDSA)
    echo "Created tree ${tree_id}"
    # Need suffix for sed -i to cope with both GNU and non-GNU (e.g. OS X) sed.
    sed -i'.bak' "1,/@TREE_ID@/s/@TREE_ID@/${tree_id}/" "${CT_CFG}"
    rm -f "${CT_CFG}.bak"
  done

  echo "CT configuration:"
  cat "${CT_CFG}"
  echo
}

# ct_stop_test closes the running processes for a CT tests.
# Assumes the following variables are set, in addition to those needed by logStopTest:
#  - CT_SERVER_PIDS  : bash array of CT HTTP server pids
ct_stop_test() {
  if [[ "${PROMETHEUS_PID}" != "" ]]; then
    kill_pid ${PROMETHEUS_PID}
  fi
  if [[ "${ETCDISCOVER_PID}" != "" ]]; then
    kill_pid ${ETCDISCOVER_PID}
  fi
  for pid in "${CT_SERVER_PIDS[@]}"; do
    echo "Stopping CT HTTP server (pid ${pid})"
    kill_pid ${pid}
  done
  log_stop_test
}
