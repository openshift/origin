#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT="${BASH_SOURCE%/*}/../.."
OS_ROOT="$(readlink -ev "${OS_ROOT}")"

source "${OS_ROOT}/hack/text.sh"
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

source "${OS_ROOT}/hack/lib/util/environment.sh"
os::util::environment::setup_time_vars

. "dind-${OPENSHIFT_INSTANCE_PREFIX}.rc"

# Cleanup cluster resources
for i in daemonsets services pods; do
    oc delete "$i" --all ||:
done >/dev/null

# ${OPENSHIFT_NUM_MINIONS} + node at master
NUM_NODES=$((${OPENSHIFT_NUM_MINIONS} + 1))

# Stop all nodes except one
for name in $NODE_MINIONS; do
    echo
    os::text::print_bold "Pause $name server"
    os::cmd::expect_success "docker pause $name"
done
echo

# Create daemonset
os::cmd::expect_success 'oc create -f examples/hello-openshift/hello-service.json'
os::cmd::expect_success 'oc create -f examples/hello-openshift/hello-daemonset.json'

docker_master="docker exec ${OPENSHIFT_INSTANCE_PREFIX}-master"
cmd="oc get pods --template '{{range.items}}{{.metadata.namespace}}/{{.metadata.name}}/{{.status.phase}}{{\"\\n\"}}{{end}}'"

# Wait for running
os::cmd::try_until_text "$cmd" "^${OC_PROJECT}/.+/Running" $((30 * $TIME_SEC))

# Check number of running and pending nodes (exclude 1 master)
os::cmd::expect_success "[[ \"\$($cmd |grep -xc '${OC_PROJECT}/.*/Running')\" == 1 ]]"
os::cmd::expect_success "[[ \"\$($cmd |grep -xc '${OC_PROJECT}/.*/Pending')\" == ${OPENSHIFT_NUM_MINIONS} ]]"

# Check answers
answers=0
while read namespace status podip; do
    os::cmd::expect_success_and_text "$docker_master curl -s http://${podip}:8080" "Hello OpenShift!"
    answers=$(($answers+1))
done <<EOF
`oc get pods --template '{{range .items}}{{.metadata.namespace}} {{.status.phase}} {{.status.podIP}}{{"\n"}}{{end}}' |
    grep -x "${OC_PROJECT} Running .*"`
EOF
os::cmd::expect_success "[[ $answers == 1 ]]"

# Start all nodes except one
for name in $NODE_MINIONS; do
    echo
    os::text::print_bold "Unpause $name server"
    os::cmd::expect_success "docker unpause $name"
done
echo

# Wait until all node start to work (include 1 master)
os::cmd::try_until_text "$cmd | grep -xc '${OC_PROJECT}/.*/Running'" "$NUM_NODES" $((30 * $TIME_SEC))

## Check answers
answers=0
while read namespace status podip; do
    os::cmd::expect_success_and_text "$docker_master curl -s http://${podip}:8080" "Hello OpenShift!"
    answers=$(($answers+1))
done <<EOF
`oc get pods --template '{{range .items}}{{.metadata.namespace}} {{.status.phase}} {{.status.podIP}}{{"\n"}}{{end}}' |
    grep -x "${OC_PROJECT} Running .*"`
EOF
os::cmd::expect_success "[[ $answers == ${NUM_NODES} ]]"
