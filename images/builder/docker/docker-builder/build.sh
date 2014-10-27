#!/bin/bash
set -uo pipefail
IFS=$'\n\t'

DOCKER_SOCKET=/var/run/docker.sock

if [ ! -e "${DOCKER_SOCKET}" ]; then
  echo "Docker socket missing at ${DOCKER_SOCKET}"
  exit 1
fi

TAG="${BUILD_TAG}"
if [ -n "${REGISTRY}" ]; then
  TAG="${REGISTRY}/${BUILD_TAG}"
fi

if [[ "${SOURCE_URI}" != "git://"* ]] && [[ "${SOURCE_URI}" != "git@"* ]]; then
  URL="${SOURCE_URI}"
  if [[ "${URL}" != "http://"* ]] && [[ "${URL}" != "https://"* ]]; then
    URL="https://${URL}"
  fi
  curl --head --silent --fail --location --max-time 16 $URL > /dev/null
  if [ $? != 0 ]; then
    echo "Not found: ${SOURCE_URI}"
    exit 1
  fi
fi

if [ -n "${SOURCE_REF}" ] || [ -n "${CONTEXT_DIR}" ]; then
  BUILD_DIR=$(mktemp --directory --suffix=docker-build)
  git clone --recursive "${SOURCE_URI}" "${BUILD_DIR}"
  if [ $? != 0 ]; then
    echo "Error trying to fetch git source: ${SOURCE_URI}"
    exit 1
  fi
  if [ -n "${SOURCE_REF}" ]; then
    pushd "${BUILD_DIR}"
    git checkout "${SOURCE_REF}"
    if [ $? != 0 ]; then
      echo "Error trying to checkout branch: ${SOURCE_REF}"
      exit 1
    fi
    popd
  fi
  if [ -n "${CONTEXT_DIR}" ] && [ ! -d "${BUILD_DIR}/${CONTEXT_DIR}" ]; then
    echo "ContextDir does not exist in the repository: ${CONTEXT_DIR}"
    exit 1
  fi
  docker build --rm -t "${TAG}" "${BUILD_DIR}/${CONTEXT_DIR}"
else
  docker build --rm -t "${TAG}" "${SOURCE_URI}"
fi

if [ -n "${REGISTRY}" ] || [ -s "/root/.dockercfg" ]; then
  docker push "${TAG}"
fi
