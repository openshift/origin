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
		os::log::warning "No \`docker\` binary found, skipping container cleanup."
		return
	fi

	os::log::info "Stopping docker containers"
	for id in $( os::cleanup::internal::list_our_containers ); do
		os::log::debug "Stopping ${id}"
		docker stop "${id}" >/dev/null
	done

	if [[ -n "${SKIP_IMAGE_CLEANUP:-}" ]]; then
		return
	fi

	os::log::info "Removing docker containers"
	for id in $( os::cleanup::internal::list_our_containers ); do
		os::log::debug "Removing ${id}"
		docker stop "${id}" >/dev/null
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
		os::log::warning "No \`docker\` binary found, skipping container cleanup."
		return
	fi

	local container_log_dir="${LOG_DIR}/containers"
	mkdir -p "${container_log_dir}"

	os::log::info "Dumping container logs to ${container_log_dir}"
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
	os::log::info "Dumping cluster events to ${ARTIFACT_DIR}/events.txt"
	local kubeconfig
	if [[ -n "${ADMIN_KUBECONFIG}" ]]; then
		kubeconfig="--config=${ADMIN_KUBECONFIG}"
	fi
	oc get events --all-namespaces ${kubeconfig} > "${ARTIFACT_DIR}/events.txt"
}
readonly -f os::cleanup::dump_events
