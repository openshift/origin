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
	os::cleanup::dump_container_logs
	os::cleanup::truncate_large_logs

	if [[ -z "${SKIP_TEARDOWN:-}" ]]; then
		os::cleanup::containers
		os::cleanup::processes
	fi
}
readonly -f os::cleanup::all

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
