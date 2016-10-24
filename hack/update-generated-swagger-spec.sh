#!/bin/bash

# Script to create latest swagger spec.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

function cleanup() {
    out=$?
    cleanup_openshift

    if [ $out -ne 0 ]; then
        echo "[FAIL] !!!!! Generate Failed !!!!"
        echo
        tail -100 "${LOG_DIR}/openshift.log"
        echo
        echo -------------------------------------
        echo
    fi
    exit $out
}

trap "exit" INT TERM
trap "cleanup" EXIT

export ALL_IP_ADDRESSES=127.0.0.1
export SERVER_HOSTNAME_LIST=127.0.0.1
export API_BIND_HOST=127.0.0.1
export API_PORT=38443
export ETCD_PORT=34001
export ETCD_PEER_PORT=37001
os::util::environment::setup_all_server_vars "generate-swagger-spec/"
os::start::configure_server

SWAGGER_SPEC_REL_DIR="${1:-}"
SWAGGER_SPEC_OUT_DIR="${OS_ROOT}/${SWAGGER_SPEC_REL_DIR}/api/swagger-spec"
mkdir -p "${SWAGGER_SPEC_OUT_DIR}"

# Start openshift
os::start::master

os::log::info "Updating ${SWAGGER_SPEC_OUT_DIR}:"

endpoint_types=("oapi" "api")
for type in "${endpoint_types[@]}"; do
    endpoints=("v1")
    for endpoint in "${endpoints[@]}"; do
        generated_file="${SWAGGER_SPEC_OUT_DIR}/${type}-${endpoint}.json"
        os::log::info "Updating ${generated_file} from /swaggerapi/${type}/${endpoint}..."
        oc get --raw "/swaggerapi/${type}/${endpoint}" --config="${MASTER_CONFIG_DIR}/admin.kubeconfig" > "${generated_file}"

        os::util::sed 's|https://127.0.0.1:38443|https://127.0.0.1:8443|g' "${generated_file}"
        printf '\n' >> "${generated_file}"
    done
done

# Swagger 2.0 / OpenAPI docs
generated_file="${SWAGGER_SPEC_OUT_DIR}/openshift-openapi-spec.json"
oc get --raw "/swagger.json" --config="${MASTER_CONFIG_DIR}/admin.kubeconfig" > "${generated_file}"

os::util::sed 's|https://127.0.0.1:38443|https://127.0.0.1:8443|g' "${generated_file}"
os::util::sed -r 's|"version": "[^\"]+"|"version": "latest"|g' "${generated_file}"
printf '\n' >> "${generated_file}"

# Copy all protobuf generated specs into the api/protobuf-spec directory
proto_spec_out_dir="${OS_ROOT}/${SWAGGER_SPEC_REL_DIR}/api/protobuf-spec"
mkdir -p "${proto_spec_out_dir}"
for proto_file in $( find "${OS_ROOT}/pkg" "${OS_ROOT}/vendor/k8s.io/kubernetes/pkg" -name generated.proto ); do
    # package declaration lines will always begin with
    # `package ` and end with `;` so to extract the
    # package name without lookarounds we can simply
    # strip characters
    package_declaration="$( grep -E '^package .+;$' "${proto_file}" )"
    package="$( echo "${package_declaration}" | cut -c 9- | cut -f 1-1 -d ';' )"

    # we want our OpenAPI documents to use underscores
    # as separators for package specifiers, not periods
    # as in the proto files
    openapi_file="${package//./_}.proto"

    cp "${proto_file}" "${proto_spec_out_dir}/${openapi_file}"
done

os::log::info "SUCCESS"
