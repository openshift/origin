#!/usr/bin/env bash

# Script to create latest swagger spec.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"
source "${OS_ROOT}/hack/local-up-master/lib.sh"

function cleanup() {
    return_code=$?
    clusterup::cleanup
    os::test::junit::generate_report
    os::cleanup::all
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

os::cleanup::tmpdir
os::util::environment::setup_all_server_vars

SWAGGER_SPEC_REL_DIR="${1:-}"
SWAGGER_SPEC_OUT_DIR="${OS_ROOT}/${SWAGGER_SPEC_REL_DIR}/api/swagger-spec"
mkdir -p "${SWAGGER_SPEC_OUT_DIR}"

# Start openshift
os::start::master

os::log::info "Updating ${SWAGGER_SPEC_OUT_DIR}:"

endpoint_types=("api")
for type in "${endpoint_types[@]}"; do
    endpoints=("v1")
    for endpoint in "${endpoints[@]}"; do
        generated_file="${SWAGGER_SPEC_OUT_DIR}/${type}-${endpoint}.json"
        os::log::info "Updating $( os::util::repository_relative_path "${generated_file}" ) from /swaggerapi/${type}/${endpoint}..."
        oc get --raw "/swaggerapi/${type}/${endpoint}" --config="${ADMIN_KUBECONFIG}" > "${generated_file}"

        os::util::sed 's|https://127.0.0.1:38443|https://127.0.0.1:8443|g' "${generated_file}"
        os::util::sed '$a\' "${generated_file}" # add eof newline if it is missing
    done
done

# Swagger 2.0 / OpenAPI docs
generated_file="${SWAGGER_SPEC_OUT_DIR}/openshift-openapi-spec.json"
oc get --raw "/swagger.json" --config="${ADMIN_KUBECONFIG}" > "${generated_file}"

os::util::sed 's|https://127.0.0.1:38443|https://127.0.0.1:8443|g' "${generated_file}"
os::util::sed -E '0,/"version":/ s|"version": "[^\"]+"|"version": "latest"|g' "${generated_file}"
os::util::sed '$a\' "${generated_file}" # add eof newline if it is missing

# Copy all protobuf generated specs into the api/protobuf-spec directory
proto_spec_out_dir="${OS_ROOT}/${SWAGGER_SPEC_REL_DIR}/api/protobuf-spec"
mkdir -p "${proto_spec_out_dir}"
for proto_file in $( find "${OS_ROOT}/vendor/github.com/openshift/api/" "${OS_ROOT}/vendor/k8s.io/kubernetes/staging/src/k8s.io/api/" -name generated.proto ); do
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

go run tools/genapidocs/genapidocs.go "${OS_ROOT}/${SWAGGER_SPEC_REL_DIR}/api/docs"
