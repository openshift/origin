#!/bin/bash

# Provides simple utility functions

# ensure_iptables_or_die tests if the testing machine has iptables available
# and in PATH. Also test whether current user has sudo privileges.
function ensure_iptables_or_die() {
	if [[ -z "$(which iptables)" ]]; then
		echo "IPTables not found - the end-to-end test requires a system with iptables for Kubernetes services."
		exit 1
	fi

	set +e

	iptables --list > /dev/null 2>&1
	if [ $? -ne 0 ]; then
		sudo iptables --list > /dev/null 2>&1
		if [ $? -ne 0 ]; then
			echo "You do not have iptables or sudo privileges. Kubernetes services will not work without iptables access.	See https://github.com/kubernetes/kubernetes/issues/1859.	Try 'sudo hack/test-end-to-end.sh'."
			exit 1
		fi
	fi

	set -e
}
readonly -f ensure_iptables_or_die

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

# dump_container_logs writes container logs to $LOG_DIR
function dump_container_logs() {
	if ! docker version >/dev/null 2>&1; then
		return
	fi

	mkdir -p ${LOG_DIR}

	os::log::info "Dumping container logs to ${LOG_DIR}"
	for container in $(docker ps -aq); do
		container_name=$(docker inspect -f "{{.Name}}" $container)
		# strip off leading /
		container_name=${container_name:1}
		if [[ "$container_name" =~ ^k8s_ ]]; then
			pod_name=$(echo $container_name | awk 'BEGIN { FS="[_.]+" }; { print $4 }')
			container_name=${pod_name}-$(echo $container_name | awk 'BEGIN { FS="[_.]+" }; { print $2 }')
		fi
		docker logs "$container" >&"${LOG_DIR}/container-${container_name}.log"
	done
}
readonly -f dump_container_logs

# delete_empty_logs deletes empty logs
function delete_empty_logs() {
	# Clean up zero byte log files
	find "${ARTIFACT_DIR}" "${LOG_DIR}" -type f -name '*.log' \( -empty \) -delete
}
readonly -f delete_empty_logs

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

# exit run if ginkgo not installed
function ensure_ginkgo_or_die() {
	which ginkgo &>/dev/null || (echo 'Run: "go get github.com/onsi/ginkgo/ginkgo"' && exit 1)
}
readonly -f ensure_ginkgo_or_die

# cleanup_openshift saves container logs, saves resources, and kills all processes and containers
function cleanup_openshift() {
	LOG_DIR="${LOG_DIR:-${BASETMPDIR}/logs}"
	ARTIFACT_DIR="${ARTIFACT_DIR:-${LOG_DIR}}"
	API_HOST="${API_HOST:-127.0.0.1}"
	API_SCHEME="${API_SCHEME:-https}"
	ETCD_PORT="${ETCD_PORT:-4001}"

	set +e
	dump_container_logs

	# pull information out of the server log so that we can get failure management in jenkins to highlight it and
	# really have it smack people in their logs.  This is a severe correctness problem
	grep -a5 "CACHE.*ALTERED" ${LOG_DIR}/openshift.log

	os::cleanup::dump_etcd

	if [[ -z "${SKIP_TEARDOWN-}" ]]; then
		os::log::info "Tearing down test"
		kill_all_processes

		if docker version >/dev/null 2>&1; then
			os::log::info "Stopping k8s docker containers"; docker ps | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker stop -t 1 >/dev/null
			if [[ -z "${SKIP_IMAGE_CLEANUP-}" ]]; then
				os::log::info "Removing k8s docker containers"; docker ps -a | awk 'index($NF,"k8s_")==1 { print $1 }' | xargs -l -r docker rm -v >/dev/null
			fi
		fi

		os::log::info "Pruning etcd data directory..."
		local sudo="${USE_SUDO:+sudo}"
		${sudo} rm -rf "${ETCD_DATA_DIR}"

		set -u
	fi

	if grep -q 'no Docker socket found' "${LOG_DIR}/openshift.log" && command -v journalctl >/dev/null 2>&1; then
		# the Docker daemon crashed, we need the logs
		journalctl --unit docker.service --since -4hours > "${LOG_DIR}/docker.log"
	fi

	delete_empty_logs
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
		-o -wholename '*/vendor/*' \
		-o -wholename './assets/bower_components/*' \
		\) -prune \
	\) -name '*.go' | sort -u
}
readonly -f find_files
