#!/bin/bash

# This command attempts to export the correct arguments for a curl client.
# Exports CURL_ARGS which should be used with curl:
#
#   $ source hack/export-certs.sh ./openshift.local.config/master/admin
#   $ curl $CURL_ARGS <a protected URL>
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

DEF="${1:-}"
CERT_DIR="${CERT_DIR:-$DEF}"
if [[ -z "${CERT_DIR}" ]]; then
    echo "Please set CERT_DIR or pass an argument corresponding to the directory to use for loading certificates"
    exit 1
fi

export CURL_CERT="${CERT_DIR}/admin.crt"
export CURL_KEY="${CERT_DIR}/admin.key"
export CURL_CA_BUNDLE="${CERT_DIR}/ca.crt"

set_curl_args
