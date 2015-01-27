#!/bin/sh

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
osc process -f examples/sample-app/docker-registry-template.json -v "OPENSHIFT_MASTER=$API_SCHEME://${CONTAINER_ACCESSIBLE_API_HOST}:${API_PORT},OPENSHIFT_CA_DATA=${OPENSHIFT_CA_DATA},OPENSHIFT_CERT_DATA=${OPENSHIFT_CERT_DATA},OPENSHIFT_KEY_DATA=${OPENSHIFT_KEY_DATA}" | osc apply -f -
