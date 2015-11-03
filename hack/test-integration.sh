#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# Go to the top of the tree.
cd "${OS_ROOT}"

os::build::setup_env

export ETCD_HOST=${ETCD_HOST:-127.0.0.1}
export ETCD_PORT=${ETCD_PORT:-44001}
export ETCD_PEER_PORT=${ETCD_PEER_PORT:-47001}


set +e

if [ "$(which etcd 2>/dev/null)" == "" ]; then
	if [[ ! -f ${OS_ROOT}/_tools/etcd/bin/etcd ]]; then
		echo "etcd must be in your PATH or installed in _tools/etcd/bin/ with hack/install-etcd.sh"
		exit 1
	fi
	export PATH="${OS_ROOT}/_tools/etcd/bin:$PATH"
fi

# Stop on any failures
set -e



function cleanup() {
	out=$?
	set +e
	if [[ $out -ne 0 && -f "${etcdlog}" ]]; then
		cat "${etcdlog}"
	fi
	kill "${ETCD_PID}" 1>&2 2>/dev/null
	echo
	echo "Complete"
	exit $out
}



package="${OS_TEST_PACKAGE:-test/integration}"
tags="${OS_TEST_TAGS:-integration !docker etcd}"

export GOMAXPROCS="$(grep "processor" -c /proc/cpuinfo 2>/dev/null || sysctl -n hw.logicalcpu 2>/dev/null || 1)"
TMPDIR=${TMPDIR:-/tmp}
export BASETMPDIR=${BASETMPDIR:-${TMPDIR}/openshift-integration}
rm -rf ${BASETMPDIR} | true
mkdir -p ${BASETMPDIR}


echo
echo "Test ${package} -tags='${tags}' ..."
echo

# setup the test dirs
export ETCD_DIR=${BASETMPDIR}/etcd
etcdlog="${ETCD_DIR}/etcd.log"
testdir="${OS_ROOT}/_output/testbin/${package}"
name="$(basename ${testdir})"
testexec="${testdir}/${name}.test"
mkdir -p "${testdir}"
mkdir -p "${ETCD_DIR}"

# build the test executable (cgo must be disabled to have the symbol table available)
pushd "${testdir}" &>/dev/null
CGO_ENABLED=0 go test -c -tags="${tags}" "${OS_GO_PACKAGE}/${package}"
popd &>/dev/null


# Start etcd
etcd -name test -data-dir ${ETCD_DIR} \
 --listen-peer-urls http://${ETCD_HOST}:${ETCD_PEER_PORT} \
 --listen-client-urls http://${ETCD_HOST}:${ETCD_PORT} \
 --initial-advertise-peer-urls http://${ETCD_HOST}:${ETCD_PEER_PORT} \
 --initial-cluster test=http://${ETCD_HOST}:${ETCD_PEER_PORT} \
 --advertise-client-urls http://${ETCD_HOST}:${ETCD_PORT} \
 &>"${etcdlog}" &
export ETCD_PID=$!

wait_for_url "http://${ETCD_HOST}:${ETCD_PORT}/version" "etcd: " 0.25 160
curl -X PUT	"http://${ETCD_HOST}:${ETCD_PORT}/v2/keys/_test"
echo

trap cleanup EXIT SIGINT

function exectest() {
	echo "Running $1..."

	result=1
	if [ -n "${VERBOSE-}" ]; then
		ETCD_PORT=${ETCD_PORT} "${testexec}" -test.v -test.run="^$1$" "${@:2}" 2>&1
		result=$?
	else
		out=$(ETCD_PORT=${ETCD_PORT} "${testexec}" -test.run="^$1$" "${@:2}" 2>&1)
		result=$?
	fi

	tput cuu 1 # Move up one line
	tput el		# Clear "running" line

	if [[ ${result} -eq 0 ]]; then
		tput setaf 2 # green
		echo "ok			$1"
		tput sgr0		# reset
		exit 0
	else
		tput setaf 1 # red
		echo "failed	$1"
		tput sgr0		# reset
		echo "${out}"
		exit 1
	fi
}

export -f exectest
export testexec
export childargs

# $1 is passed to grep -E to filter the list of tests; this may be the name of a single test,
# a fragment of a test name, or a regular expression.
#
# Examples:
#
# hack/test-integration.sh WatchBuilds
# hack/test-integration.sh Template*
# hack/test-integration.sh "(WatchBuilds|Template)"

# run each test as its own process
pushd "./${package}" &>/dev/null
time go run "${OS_ROOT}/hack/listtests.go" -prefix="${OS_GO_PACKAGE}/${package}.Test" "${testdir}" \
	| grep --color=never -E "${1-Test}" \
	| xargs -I {} -n 1 bash -c "exectest {} ${@:2}" # "${testexec}" -test.run="^{}$" "${@:2}"
popd &>/dev/null

ret=$?; ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
