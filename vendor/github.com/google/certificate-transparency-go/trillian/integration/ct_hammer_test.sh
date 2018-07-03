#!/bin/bash
set -e
. "${GOPATH}"/src/github.com/google/trillian/integration/functions.sh
INTEGRATION_DIR="$( cd "$( dirname "$0" )" && pwd )"
. "${INTEGRATION_DIR}"/ct_functions.sh

# Default to one of everything.
RPC_SERVER_COUNT=${1:-1}
LOG_SIGNER_COUNT=${2:-1}
HTTP_SERVER_COUNT=${3:-1}

go build github.com/google/certificate-transparency-go/trillian/integration/ct_hammer
ct_prep_test "${RPC_SERVER_COUNT}" "${LOG_SIGNER_COUNT}" "${HTTP_SERVER_COUNT}"

# Cleanup for the Trillian components
TO_DELETE="${TO_DELETE} ${ETCD_DB_DIR}"
TO_KILL+=(${LOG_SIGNER_PIDS[@]})
TO_KILL+=(${RPC_SERVER_PIDS[@]})
TO_KILL+=(${ETCD_PID})
TO_KILL+=(${PROMETHEUS_PID})
TO_KILL+=(${ETCDISCOVER_PID})

# Cleanup for the personality
TO_DELETE="${TO_DELETE} ${CT_CFG}"
TO_KILL+=(${CT_SERVER_PIDS[@]})

metrics_port=$(pick_unused_port)
echo "Running test(s) with metrics at localhost:${metrics_port}"
set +e
./ct_hammer --log_config "${CT_CFG}" --ct_http_servers=${CT_SERVERS} --mmd=30s --testdata_dir=${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/testdata --metrics_endpoint="localhost:${metrics_port}" --logtostderr ${HAMMER_OPTS}
RESULT=$?
set -e

ct_stop_test
TO_KILL=()

exit $RESULT
