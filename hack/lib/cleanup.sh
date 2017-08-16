#!/bin/bash

# This library holds functions that are used to clean up local
# system state after other scripts have run.

# os::cleanup::all will clean up all of the processes and data that
# a script leaves around after running. All of the sub-tasks called
# from this function should gracefully handle when they do not need
# to do anything.
#
# Globals:
#  - ARTIFACT_DIR
#  - SKIP_CLEANUP
#  - SKIP_TEARDOWN
#  - SKIP_IMAGE_CLEANUP
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::all() {
	if [[ -n "${SKIP_CLEANUP:-}" ]]; then
		os::log::warning "[CLEANUP] Skipping cleanup routines..."
		return 0
	fi

	# All of our cleanup is best-effort, so we do not care
	# if any specific step fails.
	set +o errexit

	os::log::info "[CLEANUP] Beginning cleanup routines..."
	os::cleanup::dump_events
	os::cleanup::dump_etcd
	os::cleanup::dump_container_logs
	os::cleanup::dump_pprof_output
	os::cleanup::find_cache_alterations
	os::cleanup::truncate_large_logs

	if [[ -z "${SKIP_TEARDOWN:-}" ]]; then
		os::cleanup::containers
		os::cleanup::processes
		os::cleanup::prune_etcd
	fi
}
readonly -f os::cleanup::all

# os::cleanup::dump_etcd dumps the full contents of etcd to a file.
#
# Globals:
#  - ARTIFACT_DIR
#  - API_SCHEME
#  - API_HOST
#  - ETCD_PORT
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::dump_etcd() {
	if [[ -n "${API_SCHEME:-}" && -n "${API_HOST:-}" && -n "${ETCD_PORT:-}" ]]; then
		local dump_dir="${ARTIFACT_DIR}/etcd"
		mkdir -p "${dump_dir}"
		os::log::info "[CLEANUP] Dumping etcd contents to $( os::util::repository_relative_path "${dump_dir}" )"
		os::util::curl_etcd "/v2/keys/?recursive=true" > "${dump_dir}/v2_dump.json"
		os::cleanup::internal::dump_etcd_v3 > "${dump_dir}/v3_dump.json"
	fi
}
readonly -f os::cleanup::dump_etcd

# os::cleanup::internal::dump_etcd_v3 dumps the full contents of etcd v3 to a file.
#
# Globals:
#  - ARTIFACT_DIR
#  - API_SCHEME
#  - API_HOST
#  - ETCD_PORT
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::internal::dump_etcd_v3() {
	local full_url="${API_SCHEME}://${API_HOST}:${ETCD_PORT}"

	local etcd_client_cert="${MASTER_CONFIG_DIR}/master.etcd-client.crt"
	local etcd_client_key="${MASTER_CONFIG_DIR}/master.etcd-client.key"
	local ca_bundle="${MASTER_CONFIG_DIR}/ca-bundle.crt"

	os::util::ensure::built_binary_exists 'etcdhelper' >&2

	etcdhelper --cert "${etcd_client_cert}" --key "${etcd_client_key}" \
	           --cacert "${ca_bundle}" --endpoint "${full_url}" dump
}
readonly -f os::cleanup::internal::dump_etcd_v3

# os::cleanup::prune_etcd removes the etcd data store from disk.
#
# Globals:
#  ARTIFACT_DIR
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::prune_etcd() {
	if [[ -n "${ETCD_DATA_DIR:-}" ]]; then
		os::log::info "[CLEANUP] Pruning etcd data directory"
		${USE_SUDO:+sudo} rm -rf "${ETCD_DATA_DIR}"
	fi
}
readonly -f os::cleanup::prune_etcd

# os::cleanup::containers operates on our containers to stop the containers
# and optionally remove the containers and any volumes they had attached.
#
# Globals:
#  - SKIP_IMAGE_CLEANUP
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::containers() {
	if ! os::util::find::system_binary docker >/dev/null 2>&1; then
		os::log::warning "[CLEANUP] No \`docker\` binary found, skipping container cleanup."
		return
	fi

	os::log::info "[CLEANUP] Stopping docker containers"
	for id in $( os::cleanup::internal::list_our_containers ); do
		os::log::debug "Stopping ${id}"
		docker stop "${id}" >/dev/null
	done

	if [[ -n "${SKIP_IMAGE_CLEANUP:-}" ]]; then
		return
	fi

	os::log::info "[CLEANUP] Removing docker containers"
	for id in $( os::cleanup::internal::list_our_containers ); do
		os::log::debug "Removing ${id}"
		docker rm --volumes "${id}" >/dev/null
	done
}
readonly -f os::cleanup::containers

# os::cleanup::dump_container_logs operates on k8s containers to dump any logs
# from the containers.
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::dump_container_logs() {
	if ! os::util::find::system_binary docker >/dev/null 2>&1; then
		os::log::warning "[CLEANUP] No \`docker\` binary found, skipping container log harvest."
		return
	fi

	local container_log_dir="${LOG_DIR}/containers"
	mkdir -p "${container_log_dir}"

	os::log::info "[CLEANUP] Dumping container logs to $( os::util::repository_relative_path "${container_log_dir}" )"
	for id in $( os::cleanup::internal::list_our_containers ); do
		local name; name="$( docker inspect --format '{{ .Name }}' "${id}" )"
		os::log::debug "Dumping logs for ${id} to ${name}.log"
		docker logs "${id}" >"${container_log_dir}/${name}.log" 2>&1
	done
}
readonly -f os::cleanup::dump_container_logs

# os::cleanup::internal::list_our_containers returns a space-delimited list of
# docker containers that belonged to some part of the Origin deployment.
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::internal::list_our_containers() {
	os::cleanup::internal::list_containers '^/origin$'
	os::cleanup::internal::list_k8s_containers
}
readonly -f os::cleanup::internal::list_our_containers

# os::cleanup::internal::list_k8s_containers returns a space-delimited list of
# docker containers that belonged to k8s.
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::internal::list_k8s_containers() {
	os::cleanup::internal::list_containers '^/k8s_.*'
}
readonly -f os::cleanup::internal::list_k8s_containers

# os::cleanup::internal::list_containers returns a space-delimited list of
# docker containers that match a name regex.
#
# Globals:
#  None
# Arguments:
#  1 - regex to match on the name
# Returns:
#  None
function os::cleanup::internal::list_containers() {
	local regex="$1"
	local ids;
	for short_id in $( docker ps -aq ); do
		local id; id="$( docker inspect --format '{{ .Id }}' "${short_id}" )"
		local name; name="$( docker inspect --format '{{ .Name }}' "${id}" )"
		if [[ "${name}" =~ ${regex} ]]; then
			ids+=( "${id}" )
		fi
	done

	echo "${ids[*]:+"${ids[*]}"}"
}
readonly -f os::cleanup::internal::list_containers

# os::cleanup::tmpdir performs cleanup of temp directories as a precondition for running a test. It tries to
# clean up mounts in the temp directories.
#
# Globals:
#  - BASETMPDIR
#  - USE_SUDO
# Returns:
#  None
function os::cleanup::tmpdir() {
	os::log::info "[CLEANUP] Cleaning up temporary directories"
	# ensure that the directories are clean
	if os::util::find::system_binary "findmnt" &>/dev/null; then
		for target in $( ${USE_SUDO:+sudo} findmnt --output TARGET --list ); do
			if [[ "${target}" == "${BASETMPDIR}"* ]]; then
				${USE_SUDO:+sudo} umount "${target}"
			fi
		done
	fi

	# delete any sub directory underneath BASETMPDIR
	for directory in $( find "${BASETMPDIR}" -mindepth 2 -maxdepth 2 ); do
		${USE_SUDO:+sudo} rm -rf "${directory}"
	done
}
readonly -f os::cleanup::tmpdir

# os::cleanup::dump_events dumps all the events from a cluster to a file.
#
# Globals:
#  ARTIFACT_DIR
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::dump_events() {
	os::log::info "[CLEANUP] Dumping cluster events to $( os::util::repository_relative_path "${ARTIFACT_DIR}/events.txt" )"
	local kubeconfig
	if [[ -n "${ADMIN_KUBECONFIG:-}" ]]; then
		kubeconfig="--config=${ADMIN_KUBECONFIG}"
	fi
	oc login -u system:admin ${kubeconfig:-}
	oc get events --all-namespaces ${kubeconfig:-} > "${ARTIFACT_DIR}/events.txt" 2>&1
}
readonly -f os::cleanup::dump_events

# os::cleanup::find_cache_alterations ulls information out of the server
# log so that we can get failure management in jenkins to highlight it
# and really have it smack people in their logs. This is a severe
# correctness problem.
#
# Globals:
#  - LOG_DIR
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::find_cache_alterations() {
	grep -ra5 "CACHE.*ALTERED" "${LOG_DIR}" || true
}
readonly -f os::cleanup::find_cache_alterations

# os::cleanup::dump_pprof_output dumps profiling output for the
# `openshift` binary
#
# Globals:
#  - LOG_DIR
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::dump_pprof_output() {
	if go tool -n pprof >/dev/null 2>&1 && [[ -s cpu.pprof ]]; then
		os::log::info "[CLEANUP] \`pprof\` output logged to $( os::util::repository_relative_path "${LOG_DIR}/pprof.out" )"
		go tool pprof -text "./_output/local/bin/$(os::build::host_platform)/openshift" cpu.pprof >"${LOG_DIR}/pprof.out" 2>&1
	fi
}
readonly -f os::cleanup::dump_pprof_output

# os::cleanup::truncate_large_logs truncates very large files under
# $LOG_DIR and $ARTIFACT_DIR so we do not upload them to cloud storage
# after CI runs.
#
# Globals:
#  - LOG_DIR
#  - ARTIFACT_DIR
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::truncate_large_logs() {
	local max_file_size="200M"
	os::log::info "[CLEANUP] Truncating log files over ${max_file_size}"
	for file in $(find "${ARTIFACT_DIR}" "${LOG_DIR}" -type f -name '*.log' \( -size +${max_file_size} \)); do
		mv "${file}" "${file}.tmp"
		echo "LOGFILE TOO LONG ($(du -h "${file}.tmp")), PREVIOUS BYTES TRUNCATED. LAST ${max_file_size} OF LOGFILE:" > "${file}"
		tail -c ${max_file_size} "${file}.tmp" >> "${file}"
		rm "${file}.tmp"
	done
}
readonly -f os::cleanup::truncate_large_logs

# os::cleanup::processes kills all processes created by the test
# script.
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::cleanup::processes() {
	os::log::info "[CLEANUP] Killing child processes"
	for job in $( jobs -pr ); do
		for child in $( pgrep -P "${job}" ); do
			${USE_SUDO:+sudo} kill "${child}" &> /dev/null
		done
		${USE_SUDO:+sudo} kill "${job}" &> /dev/null
	done
}
readonly -f os::cleanup::processes
