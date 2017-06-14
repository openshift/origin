#!/bin/bash

# This library holds utility functions for building container images.

# os::build::image builds an image from a directory, to a tag, with an optional dockerfile to
# use as the third argument. The environment variable OS_BUILD_IMAGE_ARGS adds additional
# options to the command. The default is to use the imagebuilder binary if it is available
# on the path with fallback to os::util::docker build if it is not available.
function os::build::image() {
  local directory=$1
  local tag=$2
  local dockerfile="${3-}"
  local extra_tag="${4-}"
  local options="${OS_BUILD_IMAGE_ARGS-}"
  local mode="${OS_BUILD_IMAGE_TYPE:-imagebuilder}"

  if [[ "${mode}" == "imagebuilder" ]]; then
    local imagebuilder
    if imagebuilder=$(os::util::find::system_binary 'imagebuilder'); then
      if [[ -n "${extra_tag}" ]]; then
        extra_tag="-t '${extra_tag}'"
      fi
      if [[ -n "${dockerfile}" ]]; then
        eval "${USE_SUDO:+sudo} ${imagebuilder} -f '${dockerfile}' -t '${tag}' ${extra_tag} ${options} '${directory}'"
        return $?
      fi
      eval "${USE_SUDO:+sudo} ${imagebuilder} -t '${tag}' ${extra_tag} ${options} '${directory}'"
      return $?
    fi

    os::log::warning "Unable to locate 'imagebuilder' on PATH, falling back to Docker build"
    # clear options since we were unable to select imagebuilder
    options=""
  fi

  if [[ -n "${dockerfile}" ]]; then
    eval "os::util::docker build -f '${dockerfile}' -t '${tag}' ${options} '${directory}'"
    if [[ -n "${extra_tag}" ]]; then
      os::util::docker tag "${tag}" "${extra_tag}"
    fi
    return $?
  fi
  eval "os::util::docker build -t '${tag}' ${options} '${directory}'"
  if [[ -n "${extra_tag}" ]]; then
    os::util::docker tag "${tag}" "${extra_tag}"
  fi
  return $?
}
readonly -f os::build::image
