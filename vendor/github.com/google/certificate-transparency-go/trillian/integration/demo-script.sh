#!/bin/bash
# This is a linear script for demonstrating a Trillian-backed CT log; its contents
# are extracted from the main trillian/integration/ct_integration_test.sh script.

if [ $(uname) == "Darwin" ]; then
  URLOPEN=open
else
  URLOPEN=xdg-open
fi
hash ${URLOPEN} 2>/dev/null || { echo >&2 "WARNING: ${URLOPEN} not found - browser windows will fail to open"; }
if [[ ! -d "${GOPATH}" ]]; then
  echo "Error: GOPATH not set"
  exit 1
fi
if [[ ${PWD} -ef ${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/integration ]]; then
  echo "Error: cannot run from directory ${PWD}; try: cd ../..; ./trillian/integration/demo-script.sh"
  exit 1
fi

echo 'Prepared before demo: edit trillian/integration/demo-script.cfg to fill in local GOPATH'
sed "s~@TESTDATA@~${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/testdata~" ${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/integration/demo-script.cfg > demo-script.cfg

echo '-----------------------------------------------'
set -x

echo 'Reset MySQL database'
yes | ${GOPATH}/src/github.com/google/trillian/scripts/resetdb.sh

echo 'Building Trillian log code'
go build github.com/google/trillian/server/trillian_log_server/
go build github.com/google/trillian/server/trillian_log_signer/

echo 'Start a Trillian Log server (do in separate terminal)'
./trillian_log_server --rpc_endpoint=localhost:6962 --http_endpoint=localhost:6963 --logtostderr &

echo 'Start a Trillian Log signer (do in separate terminal)'
./trillian_log_signer --force_master --sequencer_interval=1s --batch_size=500 --rpc_endpoint=localhost:6961 --http_endpoint=localhost:6964 --num_sequencers 2 --logtostderr &

echo 'Wait for things to come up'
sleep 8

echo 'Building provisioning tool'
go build github.com/google/trillian/cmd/createtree/

echo 'Provision a log and remember the its tree ID'
tree_id=$(./createtree --admin_server=localhost:6962 --private_key_format=PrivateKey --pem_key_path=${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/testdata/log-rpc-server.privkey.pem --pem_key_password=towel --signature_algorithm=ECDSA)
echo ${tree_id}

echo 'Manually edit CT config file to put the tree ID value in place of @TREE_ID@'
sed -i'.bak' "1,/@TREE_ID@/s/@TREE_ID@/${tree_id}/" demo-script.cfg

echo 'Building CT personality code'
go build github.com/google/certificate-transparency-go/trillian/ctfe/ct_server

echo 'Running the CT personality (do in separate terminal)'
./ct_server --log_config=demo-script.cfg --log_rpc_server=localhost:6962 --http_endpoint=localhost:6965 &
ct_pid=$!
sleep 5

echo 'Log is now accessible -- see in browser window'
${URLOPEN} http://localhost:6965/athos/ct/v1/get-sth

echo 'But is has no data, so building the Hammer test tool'
go build github.com/google/certificate-transparency-go/trillian/integration/ct_hammer

echo 'Hammer time'
./ct_hammer --log_config demo-script.cfg --ct_http_servers=localhost:6965 --mmd=30s --testdata_dir=${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/testdata --logtostderr &
hammer_pid=$!

echo 'After waiting for a while, refresh the browser window to see a bigger tree'
sleep 5
${URLOPEN} http://localhost:6965/athos/ct/v1/get-sth



sleep 10
echo 'Now lets add another log.  First kill the hammer'
kill -9 ${hammer_pid}

echo 'Provision a log and remember the its tree ID'
tree_id_2=$(./createtree --admin_server=localhost:6962 --private_key_format=PrivateKey --pem_key_path=${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/testdata/log-rpc-server.privkey.pem --pem_key_password=towel --signature_algorithm=ECDSA)
echo ${tree_id_2}

echo 'Manually edit CT config file to copy the athos config to be a second config with prefix: "porthos" and with the new tree ID'
cp demo-script.cfg  demo-script-2.cfg
cat demo-script.cfg | sed 's/athos/porthos/' | sed "s/${tree_id}/${tree_id_2}/" >> demo-script-2.cfg

echo 'Stop and restart the CT personality to use the new config (note changed --log_config)'
kill -9 ${ct_pid}
./ct_server --log_config=demo-script-2.cfg --log_rpc_server=localhost:6962 --http_endpoint=localhost:6965 &
sleep 5

echo 'See the new (empty) log'
${URLOPEN} http://localhost:6965/porthos/ct/v1/get-sth

echo 'Double Hammer time (note changed --log_config)'
./ct_hammer --log_config demo-script-2.cfg --ct_http_servers=localhost:6965 --mmd=30s --testdata_dir=${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/testdata --logtostderr &
hammer_pid=$!


sleep 30

echo 'Remember to kill off all of the jobs, so their (hard-coded) ports get freed up.  Shortcut:'
${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/integration/ct_killall.sh
echo '...but ct_killall does not kill the hammer'
killall -9 ct_hammer


# Other ideas to extend a linear demo:
#  1) Add a temporal log config, which just involves adding a fragment like the following (for 2017):
#         not_after_start {
#           seconds: 1483228800
#         }
#         not_after_limit {
#           seconds: 1514764800
#         }
#  2) Run multiple signers and use etcd to provide mastership election:
#       - install etcd with: go install ./vendor/github.com/coreos/etcd/cmd/etcd
#       - run etcd, which listens on default port :2379
#       - drop the --force_master argument to the signer
#       - add argument to the signers:  --etcd_servers=localhost:2379
#  3) Run Prometheus for metrics collection and examination (best to use top-level scripts for this):
#       - go get github.com/prometheus/prometheus/cmd/...
#       - export ETCD_DIR=${GOPATH}/bin
#       - export PROMETHEUS_DIR=${GOPATH}/bin
#       - ./trillian/integration/ct_hammer_test.sh 3 3 1
#       - open http://localhost:9090/targets to see what's being monitored
#       - open http://localhost:9090/consoles/trillian.html to see Trillian-specific metrics
