#!/bin/bash
set -e
. "${GOPATH}"/src/github.com/google/trillian/integration/functions.sh
INTEGRATION_DIR="$( cd "$( dirname "$0" )" && pwd )"
. "${INTEGRATION_DIR}"/ct_functions.sh

# Default to one of everything.
RPC_SERVER_COUNT=${1:-1}
LOG_SIGNER_COUNT=${2:-1}
HTTP_SERVER_COUNT=${3:-1}

ct_prep_test "${RPC_SERVER_COUNT}" "${LOG_SIGNER_COUNT}" "${HTTP_SERVER_COUNT}"

# Cleanup for the Trillian components
TO_DELETE="${TO_DELETE} ${ETCD_DB_DIR} ${PROMETHEUS_CFGDIR}"
TO_KILL+=(${LOG_SIGNER_PIDS[@]})
TO_KILL+=(${RPC_SERVER_PIDS[@]})
TO_KILL+=(${ETCD_PID})
TO_KILL+=(${PROMETHEUS_PID})
TO_KILL+=(${ETCDISCOVER_PID})

# Cleanup for the personality
TO_DELETE="${TO_DELETE} ${CT_CFG} ${CT_LIFECYCLE_CFG} ${CT_COMBINED_CONFIG}"
TO_KILL+=(${CT_SERVER_PIDS[@]})

COMMON_ARGS="--ct_http_servers=${CT_SERVERS} --ct_metrics_servers=${CT_METRICS_SERVERS} --testdata_dir=${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/testdata"

echo "Running test(s)"
pushd "${INTEGRATION_DIR}"
set +e
go test -v -run ".*LiveCT.*" --timeout=5m ./ --log_config "${CT_CFG}" ${COMMON_ARGS}
RESULT=$?
set -e
popd

# Integration test run failed? Clean up and exit if so
if [[ "${RESULT}" != "0" ]]; then
  ct_stop_test
  TO_KILL=()

  exit $RESULT
fi

# Now run the lifecycle test. This will use the same servers but with a
# different set of empty logs.
pushd "${INTEGRATION_DIR}"
set +e
go test -v -run ".*LiveLifecycle.*" --timeout=5m ./ --log_config "${CT_LIFECYCLE_CFG}" --admin_server="${RPC_SERVER_1}" ${COMMON_ARGS}
RESULT=$?
set -e
popd

ct_stop_test
TO_KILL=()

exit $RESULT
