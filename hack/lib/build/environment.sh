#!/bin/bash

# This script holds library functions for setting up the Docker container build environment

# os::build::environment::create creates a docker container with the default variables.
# arguments are passed directly to the container, OS_BUILD_ENV_GOLANG, OS_BUILD_ENV_IMAGE,
# and OS_RELEASE_DOCKER_ARGS can be used to customize the container. The docker socket
# is mounted by default and the output of the command is the container id.
function os::build::environment::create() {
  set -o errexit
  local release_image="${OS_BUILD_ENV_IMAGE}"
  local additional_context="${OS_BUILD_ENV_DOCKER_ARGS:-}"
  if [[ "${OS_BUILD_ENV_USE_DOCKER:-y}" == "y" ]]; then
    additional_context+="--privileged -v /var/run/docker.sock:/var/run/docker.sock"

    if [[ "${OS_BUILD_ENV_LOCAL_DOCKER:-n}" == "y" ]]; then
      # if OS_BUILD_ENV_LOCAL_DOCKER==y, add the local OS_ROOT as the bind mount to the working dir
      # and set the running user to the current user
      local workingdir
      workingdir=$( os::build::environment::release::workingdir )
      additional_context+=" -v ${OS_ROOT}:${workingdir} -u $(id -u)"
    elif [[ -n "${OS_BUILD_ENV_REUSE_VOLUME:-}" ]]; then
      # if OS_BUILD_ENV_REUSE_VOLUME is set, create a docker volume to store the working output so
      # successive iterations can reuse shared code.
      local workingdir
      workingdir=$( os::build::environment::release::workingdir )
      name="$( echo "${OS_BUILD_ENV_REUSE_VOLUME}" | tr '[:upper:]' '[:lower:]' )"
      docker volume create --name "${name}" > /dev/null
      additional_context+=" -v ${name}:${workingdir}"
    fi
  fi

  if [[ -n "${OS_BUILD_ENV_FROM_ARCHIVE-}" ]]; then
    additional_context+=" -e OS_VERSION_FILE=/tmp/os-version-defs"
  else
    additional_context+=" -e OS_VERSION_FILE="
  fi

  local args
  if [[ $# -eq 0 ]]; then
    args=( "echo" "docker create ${additional_context} ${release_image}" )
  else
    args=( "$@" )
  fi

  # Create a new container from the release environment
  docker create ${additional_context} "${release_image}" "${args[@]}"
}
readonly -f os::build::environment::create

# os::build::environment::release::workingdir calculates the working directory for the current
# release image.
function os::build::environment::release::workingdir() {
  set -o errexit
  # get working directory
  local container
  container="$(docker create "${release_image}")"
  local workingdir
  workingdir="$(docker inspect -f '{{ index . "Config" "WorkingDir" }}' "${container}")"
  docker rm "${container}" > /dev/null
  echo "${workingdir}"
}
readonly -f os::build::environment::release::workingdir

# os::build::environment::cleanup stops and removes the container named in the argument
# (unless OS_BUILD_ENV_LEAVE_CONTAINER is set, in which case it will only stop the container).
function os::build::environment::cleanup() {
  local container=$1
  docker stop --time=0 "${container}" > /dev/null || true
  if [[ -z "${OS_BUILD_ENV_LEAVE_CONTAINER:-}" ]]; then
    docker rm "${container}" > /dev/null
  fi
}
readonly -f os::build::environment::cleanup

# os::build::environment::start starts the container provided as the first argument
# using whatever content exists in the container already.
function os::build::environment::start() {
  local container=$1

  docker start "${container}" > /dev/null
  docker logs -f "${container}"

  local exitcode
  exitcode="$( docker inspect --type container -f '{{ .State.ExitCode }}' "${container}" )"

  # extract content from the image
  if [[ -n "${OS_BUILD_ENV_PRESERVE-}" ]]; then
    local workingdir
    workingdir="$(docker inspect -f '{{ index . "Config" "WorkingDir" }}' "${container}")"
    local oldIFS="${IFS}"
    IFS=:
    for path in ${OS_BUILD_ENV_PRESERVE}; do
      local parent=.
      if [[ "${path}" != "." ]]; then
        parent="$( dirname "${path}" )"
        mkdir -p "${parent}"
      fi
      docker cp "${container}:${workingdir}/${path}" "${parent}"
    done
    IFS="${oldIFS}"
  fi
  return "${exitcode}"
}
readonly -f os::build::environment::start

# os::build::environment::withsource starts the container provided as the first argument
# after copying in the contents of the current Git repository at HEAD (or, if specified,
# the ref specified in the second argument).
function os::build::environment::withsource() {
  local container=$1
  local commit=${2:-HEAD}

  if [[ -n "${OS_BUILD_ENV_LOCAL_DOCKER-}" ]]; then
    os::build::environment::start "${container}"
    return
  fi

  local workingdir
  workingdir="$(docker inspect -f '{{ index . "Config" "WorkingDir" }}' "${container}")"

  if [[ -n "${OS_BUILD_ENV_FROM_ARCHIVE-}" ]]; then
    # Generate version definitions. Tree state is clean because we are pulling from git directly.
    OS_GIT_TREE_STATE=clean os::build::get_version_vars
    os::build::save_version_vars "/tmp/os-version-defs"

    tar -cf - -C /tmp/ os-version-defs | docker cp - "${container}:/tmp"
    git archive --format=tar "${commit}" | docker cp - "${container}:${workingdir}"
    os::build::environment::start "${container}"
    return
  fi

  local excluded=()
  local oldIFS="${IFS}"
  IFS=:
  for exclude in ${OS_BUILD_ENV_EXCLUDE:-_output}; do
    excluded+=("--exclude=${exclude}")
  done
  IFS="${oldIFS}"
  if which rsync &>/dev/null && [[ -n "${OS_BUILD_ENV_REUSE_VOLUME-}" ]]; then
    local name
    name="$( echo "${OS_BUILD_ENV_REUSE_VOLUME}" | tr '[:upper:]' '[:lower:]' )"
    if ! rsync -a --blocking-io "${excluded[@]}" --delete --omit-dir-times --numeric-ids -e "docker run --rm -i -v \"${name}:${workingdir}\" --entrypoint=/bin/bash \"${OS_BUILD_ENV_IMAGE}\" -c '\$@'" . remote:"${workingdir}"; then
      # fall back to a tar if rsync is not in container
      tar -cf - "${excluded[@]}" . | docker cp - "${container}:${workingdir}"
    fi
  else
    tar -cf - "${excluded[@]}" . | docker cp - "${container}:${workingdir}"
  fi

  os::build::environment::start "${container}"
}
readonly -f os::build::environment::withsource

# os::build::environment::run launches the container with the provided arguments and
# the current commit (defaults to HEAD). The container is automatically cleaned up.
function os::build::environment::run() {
  local commit="${OS_GIT_COMMIT:-HEAD}"
  local volume="${OS_BUILD_ENV_REUSE_VOLUME:-}"
  if [[ -z "${volume}" ]]; then
    volume="origin-build-$( git rev-parse "${commit}" )"
  fi

  local container
  container="$( OS_BUILD_ENV_REUSE_VOLUME=${volume} os::build::environment::create "$@" )"
  trap "os::build::environment::cleanup ${container}" EXIT

  os::build::environment::withsource "${container}" "${commit}"
}
readonly -f os::build::environment::run