#!/bin/bash

# This library holds functions that are used to clean up local
# system state after other scripts have run.

# os::cleanup::dump_etcd dumps the full contents of etcd to a file.
#
# Globals:
#  ARTIFACT_DIR
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::dump_etcd() {
	os::log::info "Dumping etcd contents to ${ARTIFACT_DIR}/etcd_dump.json"
	os::util::curl_etcd "/v2/keys/?recursive=true" > "${ARTIFACT_DIR}/etcd_dump.json"
}

# os::cleanup::jobs cleans up running jobs spawned by this script.
# We assume that the jobs are well-formed to respond to SIGTERM and
# that they correctly forward the signal to their children.
#
# Globals:
#  - USE_SUDO
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::jobs() {
	local sudo="${USE_SUDO:+sudo}"

	os::log::info "Terminating running test jobs..."
	for i in $(jobs -pr); do
		${sudo} kill -SIGTERM "${i}"
	done
}
readonly -f os::cleanup::jobs

# dump_container_logs writes container logs to $LOG_DIR
function dump_container_logs() {
	if ! os::util::ensure::system_binary_exists 'docker'; then
		return
	fi

	os::log::info "Dumping container logs to ${LOG_DIR}"
	for container in $(docker ps -aq); do
		container_name=$(docker inspect -f "{{.Name}}" "${container}")
		# strip off leading /
		container_name=${container_name:1}
		if [[ "${container_name}" =~ ^k8s_ ]]; then
			pod_name=$(echo "${container_name}" | awk 'BEGIN { FS="[_.]+" }; { print $4 }')
			container_name=${pod_name}-$(echo "${container_name}" | awk 'BEGIN { FS="[_.]+" }; { print $2 }')
		fi
		docker logs "${container}" >&"${LOG_DIR}/container-${container_name}.log"
	done
}
readonly -f dump_container_logs

# os::cleanup::empty_logfiles deletes empty logfiles
#
# Globals:
#  - ARTIFACT_DIR
#  - LOG_DIR
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::empty_logfiles() {
	# Clean up zero byte log files
	find "${ARTIFACT_DIR}" "${LOG_DIR}" -type f -name '*.log' \( -empty \) -delete
}
readonly -f os::cleanup::empty_logfiles

# os::cleanup::large_logfiles truncates large logs
#
# Globals:
#  - ARTIFACT_DIR
#  - LOG_DIR
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::large_logfiles() {
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
readonly -f os::cleanup::large_logfiles

# os::cleanup::openshift saves container logs, saves resources, and kills all processes and containers
#
# Globals:
#  - ARTIFACT_DIR
#  - LOG_DIR
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::openshift() {
	LOG_DIR="${LOG_DIR:-${BASETMPDIR}/logs}"
	ARTIFACT_DIR="${ARTIFACT_DIR:-${LOG_DIR}}"
	API_HOST="${API_HOST:-127.0.0.1}"
	API_SCHEME="${API_SCHEME:-https}"
	ETCD_PORT="${ETCD_PORT:-4001}"

	set +e
	dump_container_logs

	# pull information out of the server log so that we can get failure management in jenkins to highlight it and
	# really have it smack people in their logs.  This is a severe correctness problem
	if [[ -f "${LOG_DIR}/openshift.log" ]]; then
		grep -a5 "CACHE.*ALTERED" "${LOG_DIR}/openshift.log"
	fi

	os::cleanup::dump_etcd

	if [[ -z "${SKIP_TEARDOWN-}" ]]; then
		os::log::info "Tearing down test"
		os::cleanup::jobs

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

	os::cleanup::empty_logfiles
	os::cleanup::large_logfiles

	os::log::info "Cleanup complete"
	set -e
}
readonly -f os::cleanup::openshift