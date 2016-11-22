#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

os::util::ensure::built_binary_exists 'dockerregistry'

url="${DOCKER_REGISTRY_URL:-localhost:5000}"
# find the first builder service account token
token="$(oc get $(oc get secrets -o name | grep builder-token | head -n 1) --template '{{ .data.token }}' | os::util::base64decode)"
echo
echo "Login with:"
echo "  docker login -p \"${token}\" -u user ${url}"
echo

REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY="${REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY:-/tmp/registry}" \
  DOCKER_REGISTRY_URL="${url}" \
	KUBECONFIG=openshift.local.config/master/openshift-registry.kubeconfig \
	dockerregistry images/dockerregistry/config.yml
