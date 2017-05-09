#!/bin/bash

# Provides simple utility functions

# kill_all_processes function will kill all
# all processes created by the test script.
function kill_all_processes() {
	local sudo="${USE_SUDO:+sudo}"

	pids=($(jobs -pr))
	for i in ${pids[@]-}; do
		pgrep -P "${i}" | xargs $sudo kill &> /dev/null
		$sudo kill ${i} &> /dev/null
	done
}
readonly -f kill_all_processes

# truncate_large_logs truncates large logs
function truncate_large_logs() {
	# Clean up large log files so they don't end up on jenkins
	local max_file_size="100M"
	local large_files=$(find "${ARTIFACT_DIR}" "${LOG_DIR}" -type f -name '*.log' \( -size +${max_file_size} \))
	for file in ${large_files}; do
		mv "${file}" "${file}.tmp"
		echo "LOGFILE TOO LONG ($(du -h "${file}.tmp")), PREVIOUS BYTES TRUNCATED. LAST ${max_file_size} OF LOGFILE:" > "${file}"
		tail -c ${max_file_size} "${file}.tmp" >> "${file}"
		rm "${file}.tmp"
	done
}
readonly -f truncate_large_logs

######
# start of common functions for extended test group's run.sh scripts
######

# cleanup_openshift saves container logs, saves resources, and kills all processes and containers
function cleanup_openshift() {
	LOG_DIR="${LOG_DIR:-${BASETMPDIR}/logs}"
	ARTIFACT_DIR="${ARTIFACT_DIR:-${LOG_DIR}}"
	API_HOST="${API_HOST:-127.0.0.1}"
	API_SCHEME="${API_SCHEME:-https}"
	ETCD_PORT="${ETCD_PORT:-4001}"

	set +e
	# pull information out of the server log so that we can get failure management in jenkins to highlight it and
	# really have it smack people in their logs.  This is a severe correctness problem
	grep -a5 "CACHE.*ALTERED" ${LOG_DIR}/openshift.log

	os::cleanup::dump_container_logs

	if [[ -z "${SKIP_TEARDOWN-}" ]]; then
		os::cleanup::dump_etcd
		os::cleanup::dump_events
		os::log::info "Tearing down test"
		kill_all_processes

		os::cleanup::containers
		os::log::info "Pruning etcd data directory..."
		local sudo="${USE_SUDO:+sudo}"
		${sudo} rm -rf "${ETCD_DATA_DIR}"

		set -u
	fi

	truncate_large_logs

	os::log::info "Cleanup complete"
	set -e
}
readonly -f cleanup_openshift

######
# end of common functions for extended test group's run.sh scripts
######

function find_files() {
	find . -not \( \
		\( \
		-wholename './_output' \
		-o -wholename './.*' \
		-o -wholename './pkg/assets/bindata.go' \
		-o -wholename './pkg/assets/*/bindata.go' \
		-o -wholename './pkg/bootstrap/bindata.go' \
		-o -wholename './openshift.local.*' \
		-o -wholename './test/extended/testdata/bindata.go' \
		-o -wholename '*/vendor/*' \
		-o -wholename './assets/bower_components/*' \
		\) -prune \
	\) -name '*.go' | sort -u
}
readonly -f find_files
