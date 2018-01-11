#!/usr/bin/env bash
#
# Extended tests for logging in using GSSAPI
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"

project_name='gssapiproxy'
test_name="test-extended/${project_name}"

os::build::setup_env

os::util::environment::use_sudo
os::util::environment::setup_time_vars
os::cleanup::tmpdir
os::util::environment::setup_all_server_vars

os::log::system::start

os::util::ensure::iptables_privileges_exist

# Always keep containers' raw output for simplicity
junit_gssapi_output="${LOG_DIR}/raw_test_output_gssapi.log"

os::test::junit::declare_suite_start "${test_name}"

os::cmd::expect_success_and_text 'oc version' 'GSSAPI Kerberos SPNEGO'

function cleanup() {
    return_code=$?
    os::test::junit::generate_report
    os::cleanup::all
    os::util::describe_return_code "${return_code}"
    exit "${return_code}"
}
trap "cleanup" EXIT

os::start::configure_server

# set up env vars
cp -R test/extended/testdata/gssapi "${BASETMPDIR}"
test_data_location="${BASETMPDIR}/gssapi"

host='gssapiproxy-server.gssapiproxy.svc.cluster.local'
realm="${host^^}"
backend='https://openshift.default.svc.cluster.local:443'

oauth_patch="$(sed "s/HOST_NAME/${host}/" "${test_data_location}/config/oauth_config.json")"
cp "${SERVER_CONFIG_DIR}/master/master-config.yaml" "${SERVER_CONFIG_DIR}/master/master-config.tmp.yaml"
oc ex config patch "${SERVER_CONFIG_DIR}/master/master-config.tmp.yaml" --patch="${oauth_patch}" > "${SERVER_CONFIG_DIR}/master/master-config.yaml"
os::start::server

export KUBECONFIG="${ADMIN_KUBECONFIG}"

os::start::registry
os::cmd::expect_success 'oc rollout status dc/docker-registry'

os::cmd::expect_success 'oc login -u system:admin'
os::cmd::expect_success "oc new-project ${project_name}"
os::cmd::expect_success "oc adm policy add-scc-to-user anyuid -z default -n ${project_name}"

# create all the resources we need
os::cmd::expect_success "oc create -f '${test_data_location}/proxy'"

# kick off a build and wait for it to finish
os::cmd::expect_success "oc set env dc/gssapiproxy-server HOST='${host}' REALM='${realm}' BACKEND='${backend}'"
os::cmd::expect_success "oc start-build --from-dir='${test_data_location}/proxy' --follow gssapiproxy"

os_images=(fedora ubuntu)

for os_image in "${os_images[@]}"; do

    pushd "${test_data_location}/${os_image}" > /dev/null

        pushd base > /dev/null
            os::cmd::expect_success "cp '$(which oc)' ."
            os::cmd::expect_success "cp -R '${OS_ROOT}/hack' ."
            os::cmd::expect_success 'cp ../../scripts/test-wrapper.sh .'
            os::cmd::expect_success 'cp ../../scripts/gssapi-tests.sh .'
            os::cmd::expect_success 'cp ../../config/kubeconfig .'
            os::cmd::expect_success "docker build --build-arg REALM='${realm}' --build-arg HOST='${host}' -t 'docker.io/${project_name}/${os_image}-gssapi-base:latest' ."
        popd > /dev/null

        pushd kerberos > /dev/null
            os::cmd::expect_success "docker build -t 'docker.io/${project_name}/${os_image}-gssapi-kerberos:latest' ."
        popd > /dev/null

        pushd kerberos_configured > /dev/null
            os::cmd::expect_success "docker build -t 'docker.io/${project_name}/${os_image}-gssapi-kerberos-configured:latest' ."
        popd > /dev/null

    popd > /dev/null

done

function update_auth_proxy_config() {
    local server_config="${1}"
    local spec='{.items[0].spec.containers[0].env[?(@.name=="SERVER")].value}'
    spec+='_'
    spec+='{.items[0].status.conditions[?(@.type=="Ready")].status}'

    os::cmd::expect_success "oc set env dc/gssapiproxy-server SERVER='${server_config}'"
    os::cmd::try_until_text "oc get pods -l deploymentconfig=gssapiproxy-server -o jsonpath='${spec}'" "^${server_config}_True$" $(( 10 * minute ))
}

function run_gssapi_tests() {
    local image_name="${1}"
    local server_config="${2}"
    local container_exit_code_jsonpath='{.status.containerStatuses[0].state.terminated.exitCode}'
    local pod_log_location="${LOG_DIR}/${image_name}-${server_config}.log"
    oc run "${image_name}"                                    \
            --image="docker.io/${project_name}/${image_name}" \
            --generator=run-pod/v1 --restart=Never --attach   \
            --env=SERVER="${server_config}"                   \
            1> "${pod_log_location}"                          \
            2>> "${junit_gssapi_output}"
    # Lots of checks to really make sure that the tests ran successfully
    os::cmd::expect_success_and_text "cat ${pod_log_location}" 'SUCCESS'
    os::cmd::expect_success_and_not_text "cat ${pod_log_location}" 'FAILURE'
    os::cmd::expect_success_and_text "cat ${pod_log_location}" "Finished running test-extended/gssapiproxy-tests/${image_name}-CLIENT_[[:upper:]_]+-${server_config}$"
    os::cmd::try_until_text "oc get pod '${image_name}' -o jsonpath='${container_exit_code_jsonpath}'" '0' # kubelet takes time to update status
    os::cmd::expect_success "oc delete pod '${image_name}'"
}

for server_config in SERVER_GSSAPI_ONLY SERVER_GSSAPI_BASIC_FALLBACK; do

    update_auth_proxy_config "${server_config}"

    for os_image in "${os_images[@]}"; do

        run_gssapi_tests "${os_image}-gssapi-base" "${server_config}"

        run_gssapi_tests "${os_image}-gssapi-kerberos" "${server_config}"

        run_gssapi_tests "${os_image}-gssapi-kerberos-configured" "${server_config}"

    done

done

os::test::junit::declare_suite_end
