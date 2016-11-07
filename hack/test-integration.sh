#!/bin/bash
STARTTIME=$(date +%s)
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

export API_SCHEME="http"
export API_BIND_HOST="127.0.0.1"
os::util::environment::setup_all_server_vars "test-integration/"

function cleanup() {
	out=$?
	set +e

	echo "Complete"
	exit $out
}

trap cleanup EXIT SIGINT

export GOMAXPROCS="$(grep "processor" -c /proc/cpuinfo 2>/dev/null || sysctl -n hw.logicalcpu 2>/dev/null || 1)"

# Internalize environment variables we consume and default if they're not set
package="${OS_TEST_PACKAGE:-test/integration}"
name="$(basename ${package})"
dlv_debug="${DLV_DEBUG:-}"
verbose="${VERBOSE:-}"

# build the test executable (cgo must be disabled to have the symbol table available)
if [[ -n "${OPENSHIFT_SKIP_BUILD:-}" ]]; then
  os::log::warn "Skipping build due to OPENSHIFT_SKIP_BUILD"
else
	CGO_ENABLED=0 "${OS_ROOT}/hack/build-go.sh" "${package}/${name}.test" -installsuffix=cgo
fi
testexec="$(pwd)/$(os::build::find-binary "${name}.test")"

os::log::system::start

function exectest() {
	echo "Running $1..."

	export TEST_ETCD_DIR="${TMPDIR:-/tmp}/etcd-${1}"
	rm -fr "${TEST_ETCD_DIR}"
	mkdir -p "${TEST_ETCD_DIR}"
	result=1
	if [[ -n "${dlv_debug}" ]]; then
		# run tests using delve debugger
		dlv exec "${testexec}" -- -test.run="^$1$" "${@:2}"
		result=$?
		out=
	elif [[ -n "${verbose}" ]]; then
		# run tests with extra verbosity
		out=$("${testexec}" -vmodule=*=5 -test.v -test.timeout=4m -test.run="^$1$" "${@:2}" 2>&1)
		result=$?
	else
		# run tests normally
		out=$("${testexec}" -test.timeout=4m -test.run="^$1$" "${@:2}" 2>&1)
		result=$?
	fi

	os::text::clear_last_line

	if [[ ${result} -eq 0 ]]; then
		os::text::print_green "ok      $1"
		# Remove the etcd directory to cleanup the space.
		rm -rf "${TEST_ETCD_DIR}"
		exit 0
	else
		os::text::print_red "failed  $1"
		echo "${out:-}"

		exit 1
	fi
}

export -f exectest
export testexec
export childargs

loop="${TIMES:-1}"
# $1 is passed to grep -E to filter the list of tests; this may be the name of a single test,
# a fragment of a test name, or a regular expression.
#
# Examples:
#
# hack/test-integration.sh WatchBuilds
# hack/test-integration.sh Template*
# hack/test-integration.sh "(WatchBuilds|Template)"
tests=( $(go run "${OS_ROOT}/hack/listtests.go" -prefix="${OS_GO_PACKAGE}/${package}.Test" "${testexec}" | grep -E "${1-Test}") )
# run each test as its own process
ret=0
pushd "${OS_ROOT}/${package}" &>/dev/null
for test in "${tests[@]}"; do
	for((i=0;i<${loop};i+=1)); do
		if ! (exectest "${test}" ${@:2}); then
			ret=1
		fi
	done
done
popd &>/dev/null

ENDTIME=$(date +%s); echo "$0 took $(($ENDTIME - $STARTTIME)) seconds"; exit "$ret"
