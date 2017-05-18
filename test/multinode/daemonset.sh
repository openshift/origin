#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT="${BASH_SOURCE%/*}/../.."
OS_ROOT="$(readlink -ev "${OS_ROOT}")"

source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

source "${OS_ROOT}/hack/lib/util/environment.sh"
os::util::environment::setup_time_vars

# Cleanup cluster resources
for i in daemonsets services pods; do
    oc delete "$i" --all ||:
done >/dev/null

# Stop all nodes except one
for i in `seq 2 ${CLUSTER_NODES}`; do
    echo
    echo "[INFO] Pause node-$i server"
    os::cmd::expect_success "docker pause node-$i"
done
echo

# Create daemonset
os::cmd::expect_success 'oc create -f examples/hello-openshift/hello-service.json'
os::cmd::expect_success 'oc create -f examples/hello-openshift/hello-daemonset.json'

cmd="oc get pods --template '{{range.items}}{{.metadata.namespace}}/{{.metadata.name}}/{{.status.phase}}{{\"\\n\"}}{{end}}'"

# Wait for running
os::cmd::try_until_text "$cmd" "^${OC_PROJECT}/.+/Running" $((30 * $TIME_SEC))

# Check number of running and pending nodes
os::cmd::expect_success "[[ \"\$($cmd |grep -xc '${OC_PROJECT}/.*/Running')\" == \"1\" ]]"
os::cmd::expect_success "[[ \"\$($cmd |grep -xc '${OC_PROJECT}/.*/Pending')\" == \"$((${CLUSTER_NODES} - 1))\" ]]"

# Check answers
answers=0
while read namespace status podip; do
    os::cmd::expect_success_and_text "curl -s http://${podip}:8080" "Hello OpenShift!"
    answers=$(($answers+1))
done <<EOF
`oc get pods --template '{{range .items}}{{.metadata.namespace}} {{.status.phase}} {{.status.podIP}}{{"\n"}}{{end}}' |
    grep -x "${OC_PROJECT} Running .*"`
EOF
os::cmd::expect_success "[[ $answers == 1 ]]"

# Start all nodes except one
for i in `seq 2 ${CLUSTER_NODES}`; do
    echo
    echo "[INFO] Unpause node-$i server"
    os::cmd::expect_success "docker unpause node-$i"
done
echo

# Wait until all node start to work
os::cmd::try_until_text "$cmd | grep -xc '${OC_PROJECT}/.*/Running'" "${CLUSTER_NODES}" $((30 * $TIME_SEC))

## Check answers
answers=0
while read namespace status podip; do
    os::cmd::expect_success_and_text "curl -s http://${podip}:8080" "Hello OpenShift!"
    answers=$(($answers+1))
done <<EOF
`oc get pods --template '{{range .items}}{{.metadata.namespace}} {{.status.phase}} {{.status.podIP}}{{"\n"}}{{end}}' |
    grep -x "${OC_PROJECT} Running .*"`
EOF
os::cmd::expect_success "[[ $answers == ${CLUSTER_NODES} ]]"
