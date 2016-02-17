#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/text.sh"
source "${OS_ROOT}/hack/lib/log.sh"
source "${OS_ROOT}/hack/lib/util/environment.sh"
os::log::install_errexit

# Go to the top of the tree.
cd "${OS_ROOT}"

os::build::setup_env

export API_SCHEME="http"
export API_BIND_HOST="127.0.0.1"
export ETCD_PORT=${ETCD_PORT:-44001}
export ETCD_PEER_PORT=${ETCD_PEER_PORT:-47001}
os::util::environment::setup_all_server_vars "test-integration/"
reset_tmp_dir

function cleanup() {
	out=$?
	set +e

	cleanup_openshift
	echo "Complete"
	exit $out
}

trap cleanup EXIT SIGINT

package="${OS_TEST_PACKAGE:-test/integration}"

if docker version >/dev/null 2>&1; then
	tags="${OS_TEST_TAGS:-integration docker etcd}"
else
	echo "++ Docker not available, running only integration tests without the 'docker' tag"
	tags="${OS_TEST_TAGS:-integration !docker etcd}"
fi

export GOMAXPROCS="$(grep "processor" -c /proc/cpuinfo 2>/dev/null || sysctl -n hw.logicalcpu 2>/dev/null || 1)"

echo
echo "Test ${package} -tags='${tags}' ..."
echo

# setup the test dirs
testdir="${OS_ROOT}/_output/testbin/${package}"
name="$(basename ${testdir})"
testexec="${testdir}/${name}.test"
mkdir -p "${testdir}"

# build the test executable (cgo must be disabled to have the symbol table available)
pushd "${testdir}" &>/dev/null
echo "Building test executable..."
CGO_ENABLED=0 go test -c -tags="${tags}" "${OS_GO_PACKAGE}/${package}"
popd &>/dev/null

os::log::start_system_logger

configure_os_server
openshift start etcd --config=${MASTER_CONFIG_DIR}/master-config.yaml &> ${LOG_DIR}/etcd.log &

wait_for_url "http://${API_HOST}:${ETCD_PORT}/version" "etcd: " 0.25 160
curl -X PUT	"http://${API_HOST}:${ETCD_PORT}/v2/keys/_test"
echo

function exectest() {
	echo "Running $1..."

	result=1
	if [ -n "${VERBOSE-}" ]; then
		ETCD_PORT=${ETCD_PORT} "${testexec}" -vmodule=*=5 -test.v -test.timeout=4m -test.run="^$1$" "${@:2}" 2>&1
		result=$?
	else
		out=$(ETCD_PORT=${ETCD_PORT} "${testexec}" -test.timeout=4m -test.run="^$1$" "${@:2}" 2>&1)
		result=$?
	fi

	os::text::clear_last_line

	if [[ ${result} -eq 0 ]]; then
		os::text::print_green "ok      $1"
		exit 0
	else
		os::text::print_red "failed  $1"
		echo "${out}"

		# dump etcd for failing test
		echo "[INFO] Dumping etcd contents to ${ARTIFACT_DIR}/$1-etcd_dump.json"
		curl -L -s "${API_SCHEME}://${API_HOST}:${ETCD_PORT}/v2/keys/?recursive=true" > "${ARTIFACT_DIR}/$1-etcd_dump.json"
		echo

		exit 1
	fi
}

export -f exectest
export testexec
export childargs

loop="${TIMES:-1}"
pushd "./${package}" &>/dev/null
# $1 is passed to grep -E to filter the list of tests; this may be the name of a single test,
# a fragment of a test name, or a regular expression.
#
# Examples:
#
# hack/test-integration.sh WatchBuilds
# hack/test-integration.sh Template*
# hack/test-integration.sh "(WatchBuilds|Template)"
tests=( $(go run "${OS_ROOT}/hack/listtests.go" -prefix="${OS_GO_PACKAGE}/${package}.Test" "${testdir}" | grep -E "${1-Test}") )
# run each test as its own process
ret=0
for test in "${tests[@]}"; do
	for((i=0;i<${loop};i+=1)); do
		if ! (exectest "${test}" ${@:2}); then
			ret=1
		fi
	done
done
popd &>/dev/null

ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
