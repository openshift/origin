#!/bin/sh

# This uses osc to bootstrap a Docker registry image as a pod under a running OpenShift.
# It uses the key/certs in the directory specified by CERT_DIR to configure the registry
# for connecting securely to the OpenShift master's.
#
# To use key/certs generated automatically by "openshift start", look for the
# openshift.local.certificates/master/ directory underneath where it was started.
# For instance, if the openshift home directory is /var/lib/openshift, then run:
# CERT_DIR=/var/lib/openshift/openshift.local.certificates/master hack/install-registry.sh
#
# You may also need to set KUBERNETES_MASTER if the master is not listening at https://localhost:8443/

if [ -z "${CERT_DIR}" ]; then
  echo "You have to set the CERT_DIR environment variable to point into master certificate"
  echo "Example:"
  echo "$ CERT_DIR='/var/lib/openshift/openshift.local.certificates/master' hack/install-registry.sh"
  exit 1
fi

set -o errexit
set -o nounset
set -o pipefail

API_PORT="${API_PORT:-8443}"
API_SCHEME="${API_SCHEME:-https}"
API_HOST="${API_HOST:-localhost}"

# use the docker bridge ip address until there is a good way to get the auto-selected address from master
# this address is considered stable
# Used by the docker-registry and the router pods to call back to the API
CONTAINER_ACCESSIBLE_API_HOST="${CONTAINER_ACCESSIBLE_API_HOST:-172.17.42.1}"

# Set KUBERNETES_MASTER for osc
KUBERNETES_MASTER="${KUBERNETES_MASTER:-${API_SCHEME}://${API_HOST}:${API_PORT}}"
if [[ "${API_SCHEME}" == "https" ]]; then
	# Read client cert data in to send to containerized components
	OPENSHIFT_CA_DATA="$(cat "${CERT_DIR}/root.crt")"
	OPENSHIFT_CERT_DATA="$(cat "${CERT_DIR}/cert.crt")"
	OPENSHIFT_KEY_DATA="$(cat "${CERT_DIR}/key.key")"
else
	OPENSHIFT_CA_DATA=""
	OPENSHIFT_CERT_DATA=""
	OPENSHIFT_KEY_DATA=""
fi
export KUBERNETES_MASTER

# Deploy private docker registry
echo "[INFO] Submitting docker-registry template file for processing"
osc process -f examples/sample-app/docker-registry-template.json -v "OPENSHIFT_MASTER=$API_SCHEME://${CONTAINER_ACCESSIBLE_API_HOST}:${API_PORT},OPENSHIFT_CA_DATA=${OPENSHIFT_CA_DATA},OPENSHIFT_CERT_DATA=${OPENSHIFT_CERT_DATA},OPENSHIFT_KEY_DATA=${OPENSHIFT_KEY_DATA}" | osc create -f -
