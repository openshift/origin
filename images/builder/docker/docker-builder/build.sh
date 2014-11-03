#!/bin/bash
set -o pipefail
IFS=$'\n\t'

DOCKER_SOCKET=/var/run/docker.sock

if [ ! -e "${DOCKER_SOCKET}" ]; then
  echo "Docker socket missing at ${DOCKER_SOCKET}"
  exit 1
fi

TAG="${BUILD_TAG}"
if [ -n "${REGISTRY}" ]; then
  TAG="${REGISTRY}/${BUILD_TAG}"
elif [ -n "${DOCKER_REGISTRY}" ]; then
  TAG="${DOCKER_REGISTRY}/${BUILD_TAG}"
fi

# backwards compatibility for older openshift versions that passed docker context instead of source_uri
if [ -n "${DOCKER_CONTEXT_URL}" ]; then
  SOURCE_URI=${DOCKER_CONTEXT_URL}
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
  pushd "${BUILD_DIR}"
  if [ -n "${SOURCE_REF}" ]; then
    git checkout "${SOURCE_REF}"
    if [ $? != 0 ]; then
      echo "Error trying to checkout branch: ${SOURCE_REF}"
      exit 1
    fi
  fi
  if [ -n "${SOURCE_ID}" ]; then
    git branch --contains ${SOURCE_ID} | grep ${SOURCE_REF}
    if [ $? != 0 ]; then
      echo "Branch '${SOURCE_REF}' does not contain commit: ${SOURCE_ID}"
      exit 1
    fi
  fi
  popd
  if [ -n "${CONTEXT_DIR}" ] && [ ! -d "${BUILD_DIR}/${CONTEXT_DIR}" ]; then
    echo "ContextDir does not exist in the repository: ${CONTEXT_DIR}"
    exit 1
  fi
  docker build --rm -t "${TAG}" "${BUILD_DIR}/${CONTEXT_DIR}"
else
  docker build --rm -t "${TAG}" "${SOURCE_URI}"
fi

if [ -n "${REGISTRY}" ] || [ -n "${DOCKER_REGISTRY}" ] || [ -s "/root/.dockercfg" ]; then
  docker push "${TAG}"
fi
