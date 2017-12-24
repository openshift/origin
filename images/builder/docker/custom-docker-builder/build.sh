#!/bin/bash

# See https://docs.openshift.org/latest/creating_images/custom.html#custom-builder-image
# for the list of environment variables set by OpenShift before the custom
# builder image is run.
#
# Although set as part of the API, the environment variables
# SOURCE_REPOSITORY/SOURCE_URI, SOURCE_CONTEXT_DIR and SOURCE_REF can also be
# derived from the BUILD environment variable using a tool such as `jq`
# (https://stedolan.github.io/jq/).  (Note: you would need to include the `jq`
# binary in your custom builder image).  If necessary, this technique can be
# used for extracting other values from the BUILD json from a shell script.
#
# SOURCE_REPOSITORY=$(jq -nr '(env.BUILD|fromjson).spec.source.git.uri')
# SOURCE_URI=$(jq -nr '(env.BUILD|fromjson).spec.source.git.uri')
# SOURCE_CONTEXT_DIR=$(jq -nr '(env.BUILD|fromjson).spec.source.contextDir')
# SOURCE_REF=$(jq -nr '(env.BUILD|fromjson).spec.source.git.ref')

set -o pipefail
IFS=$'\n\t'

DOCKER_SOCKET=/var/run/docker.sock

if [ ! -e "${DOCKER_SOCKET}" ]; then
  echo "Docker socket missing at ${DOCKER_SOCKET}"
  exit 1
fi

if [ -n "${OUTPUT_IMAGE}" ]; then
  TAG="${OUTPUT_REGISTRY}/${OUTPUT_IMAGE}"
fi

if [[ "${SOURCE_REPOSITORY}" != "git://"* ]] && [[ "${SOURCE_REPOSITORY}" != "git@"* ]]; then
  URL="${SOURCE_REPOSITORY}"
  if [[ "${URL}" != "http://"* ]] && [[ "${URL}" != "https://"* ]]; then
    URL="https://${URL}"
  fi
  curl --head --silent --fail --location --max-time 16 $URL > /dev/null
  if [ $? != 0 ]; then
    echo "Could not access source url: ${SOURCE_REPOSITORY}"
    exit 1
  fi
fi

if [ -n "${SOURCE_REF}" ]; then
  BUILD_DIR=$(mktemp --directory)
  git clone --recursive "${SOURCE_REPOSITORY}" "${BUILD_DIR}"
  if [ $? != 0 ]; then
    echo "Error trying to fetch git source: ${SOURCE_REPOSITORY}"
    exit 1
  fi
  pushd "${BUILD_DIR}"
  git checkout "${SOURCE_REF}"
  if [ $? != 0 ]; then
    echo "Error trying to checkout branch: ${SOURCE_REF}"
    exit 1
  fi
  popd
  docker build --rm -t "${TAG}" "${BUILD_DIR}"
else
  docker build --rm -t "${TAG}" "${SOURCE_REPOSITORY}"
fi

if [[ -d /var/run/secrets/openshift.io/push ]] && [[ ! -e /root/.dockercfg ]]; then
  cp /var/run/secrets/openshift.io/push/.dockercfg /root/.dockercfg
fi

if [ -n "${OUTPUT_IMAGE}" ] || [ -s "/root/.dockercfg" ]; then
  docker push "${TAG}"
fi
