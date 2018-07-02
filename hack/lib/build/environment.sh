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

  local workingdir
  workingdir=$( os::build::environment::release::workingdir )
  additional_context+=" -w ${workingdir}"

  if [[ "${OS_BUILD_ENV_USE_DOCKER:-y}" == "y" ]]; then
    additional_context+=" --privileged -v /var/run/docker.sock:/var/run/docker.sock"

    if [[ "${OS_BUILD_ENV_LOCAL_DOCKER:-n}" == "y" ]]; then
      # if OS_BUILD_ENV_LOCAL_DOCKER==y, add the local OS_ROOT as the bind mount to the working dir
      # and set the running user to the current user
      additional_context+=" -v ${OS_ROOT}:${workingdir} -u $(id -u)"
    elif [[ -n "${OS_BUILD_ENV_VOLUME:-}" ]]; then
      if docker volume inspect "${OS_BUILD_ENV_VOLUME}" >/dev/null 2>&1; then
        os::log::debug "Re-using volume ${OS_BUILD_ENV_VOLUME}"
      else
        # if OS_BUILD_ENV_VOLUME is set and no volume already exists, create a docker volume to
        # store the working output so successive iterations can reuse shared code.
        os::log::debug "Creating volume ${OS_BUILD_ENV_VOLUME}"
        docker volume create --name "${OS_BUILD_ENV_VOLUME}" > /dev/null
      fi

      if [[ -n "${OS_BUILD_ENV_TMP_VOLUME:-}" ]]; then
        if docker volume inspect "${OS_BUILD_ENV_TMP_VOLUME}" >/dev/null 2>&1; then
          os::log::debug "Re-using volume ${OS_BUILD_ENV_TMP_VOLUME}"
        else
          # if OS_BUILD_ENV_VOLUME is set and no volume already exists, create a docker volume to
          # store the working output so successive iterations can reuse shared code.
          os::log::debug "Creating volume ${OS_BUILD_ENV_TMP_VOLUME}"
          docker volume create --name "${OS_BUILD_ENV_TMP_VOLUME}" >/dev/null
        fi
        additional_context+=" -v ${OS_BUILD_ENV_TMP_VOLUME}:/tmp"
      fi
      additional_context+=" -v ${OS_BUILD_ENV_VOLUME}:${workingdir}"
    fi
  fi

  if [[ -n "${OS_BUILD_ENV_FROM_ARCHIVE-}" ]]; then
    additional_context+=" -e OS_VERSION_FILE=/tmp/os-version-defs"
  else
    additional_context+=" -e OS_VERSION_FILE="
  fi

  declare -a cmd=( )
  declare -a env=( )
  local prefix=1
  for arg in "${@:1}"; do
    if [[ "${arg}" != *"="* ]]; then
      prefix=0
    fi
    if [[ "${prefix}" -eq 1 ]]; then
      env+=( "-e" "${arg}" )
    else
      cmd+=( "${arg}" )
    fi
  done
  if [[ -t 0 ]]; then
    if [[ "${#cmd[@]}" -eq 0 ]]; then
      cmd=( "/bin/sh" )
    fi
    if [[ "${cmd[0]}" == "/bin/sh" || "${cmd[0]}" == "/bin/bash" ]]; then
      additional_context+=" -it"
    else
      # container exit races with log collection so we
      # need to sleep at the end but preserve the exit
      # code of whatever the user asked for us to run
      cmd=( '/bin/bash' '-c' "${cmd[*]}; return_code=\$?; sleep 1; exit \${return_code}" )
    fi
  fi

  # Create a new container from the release environment
  os::log::debug "Creating container: \`docker create ${additional_context} ${env[@]+"${env[@]}"} ${release_image} ${cmd[@]+"${cmd[@]}"}"
  docker create ${additional_context} "${env[@]+"${env[@]}"}" "${release_image}" "${cmd[@]+"${cmd[@]}"}"
}
readonly -f os::build::environment::create

# os::build::environment::release::workingdir calculates the working directory for the current
# release image.
function os::build::environment::release::workingdir() {
  if [[ -n "${OS_BUILD_ENV_WORKINGDIR-}" ]]; then
    echo "${OS_BUILD_ENV_WORKINGDIR}"
    return 0
  fi
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
  local volume=$2
  local tmp_volume=$3
  os::log::debug "Stopping container ${container}"
  docker stop --time=0 "${container}" > /dev/null || true
  if [[ -z "${OS_BUILD_ENV_LEAVE_CONTAINER:-}" ]]; then
    os::log::debug "Removing container ${container}"
    docker rm "${container}" > /dev/null

    if [[ -z "${OS_BUILD_ENV_REUSE_TMP_VOLUME:-}" ]]; then
      os::log::debug "Removing tmp build volume"
      os::build::environment::remove_volume "${tmp_volume}"
    fi
    if [[ -n "${OS_BUILD_ENV_CLEAN_BUILD_VOLUME:-}" ]]; then
      os::log::debug "Removing build volume"
      os::build::environment::remove_volume "${volume}"
    fi
  fi
}
readonly -f os::build::environment::cleanup

# os::build::environment::start starts the container provided as the first argument
# using whatever content exists in the container already.
function os::build::environment::start() {
  local container=$1

  os::log::debug "Starting container ${container}"
  if [[ "$( docker inspect --type container -f '{{ .Config.OpenStdin }}' "${container}" )" == "true" ]]; then
    docker start -ia "${container}"
  else
    docker start "${container}" > /dev/null
    os::log::debug "Following container logs"
    docker logs -f "${container}"
  fi

  local exitcode
  exitcode="$( docker inspect --type container -f '{{ .State.ExitCode }}' "${container}" )"

  os::log::debug "Container exited with ${exitcode}"

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
      os::log::debug "Copying from ${container}:${workingdir}/${path} to ${parent}"
      if ! output="$( docker cp "${container}:${workingdir}/${path}" "${parent}" 2>&1 )"; then
        os::log::warning "Copying ${path} from the container failed!"
        os::log::warning "${output}"
      fi
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
    OS_GIT_TREE_STATE=clean os::build::version::get_vars
    os::build::version::save_vars > "/tmp/os-version-defs"

    os::log::debug "Generating source code archive"
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
  if which rsync &>/dev/null && [[ -n "${OS_BUILD_ENV_VOLUME-}" ]]; then
    os::log::debug "Syncing source using \`rsync\`"
    if ! rsync -a --blocking-io "${excluded[@]}" --delete --omit-dir-times --numeric-ids -e "docker run --rm -i -v \"${OS_BUILD_ENV_VOLUME}:${workingdir}\" --entrypoint=/bin/bash \"${OS_BUILD_ENV_IMAGE}\" -c '\$@'" . remote:"${workingdir}"; then
      os::log::debug "Falling back to \`tar\` and \`docker cp\` as \`rsync\` is not in container"
      tar -cf - "${excluded[@]}" . | docker cp - "${container}:${workingdir}"
    fi
  else
    os::log::debug "Syncing source using \`tar\` and \`docker cp\`"
    tar -cf - "${excluded[@]}" . | docker cp - "${container}:${workingdir}"
  fi

  os::build::environment::start "${container}"
}
readonly -f os::build::environment::withsource

function os::build::environment::volume_name() {
  local prefix=$1
  local commit=$2
  local volume=$3

  if [[ -z "${volume}" ]]; then
    volume="${prefix}-$( git rev-parse "${commit}" )"
  fi

  echo "${volume}" | tr '[:upper:]' '[:lower:]'
}
readonly -f os::build::environment::volume_name

function os::build::environment::remove_volume() {
  local volume=$1

  if docker volume inspect "${volume}" >/dev/null 2>&1; then
    os::log::debug "Removing volume ${volume}"
    docker volume rm "${volume}" >/dev/null
  fi
}
readonly -f os::build::environment::remove_volume

# os::build::environment::run launches the container with the provided arguments and
# the current commit (defaults to HEAD). The container is automatically cleaned up.
function os::build::environment::run() {
  local commit="${OS_GIT_COMMIT:-HEAD}"
  local volume
  local tmp_volume

  volume="$( os::build::environment::volume_name "origin-build" "${commit}" "${OS_BUILD_ENV_REUSE_VOLUME:-}" )"
  tmp_volume="$( os::build::environment::volume_name "origin-build-tmp" "${commit}" "${OS_BUILD_ENV_REUSE_TMP_VOLUME:-}" )"

  export OS_BUILD_ENV_VOLUME="${volume}"
  export OS_BUILD_ENV_TMP_VOLUME="${tmp_volume}"

  if [[ -n "${OS_BUILD_ENV_VOLUME_FORCE_NEW:-}" ]]; then
    os::build::environment::remove_volume "${volume}"
    os::build::environment::remove_volume "${tmp_volume}"
  fi

  if [[ -n "${OS_BUILD_ENV_PULL_IMAGE:-}" ]]; then
    os::log::info "Pulling the ${OS_BUILD_ENV_IMAGE} image to update it..."
    docker pull "${OS_BUILD_ENV_IMAGE}"
  fi

  os::log::debug "Using commit ${commit}"
  os::log::debug "Using volume ${volume}"
  os::log::debug "Using tmp volume ${tmp_volume}"

  local container
  container="$( os::build::environment::create "$@" )"
  trap "os::build::environment::cleanup ${container} ${volume} ${tmp_volume}" EXIT

  os::log::debug "Using container ${container}"

  os::build::environment::withsource "${container}" "${commit}"
}
readonly -f os::build::environment::run
