#!/bin/bash
#
# Runs the federation tests against a standard started server.
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"
source "${OS_ROOT}/test/extended/setup.sh"

os::test::extended::setup
os::util::ensure::built_binary_exists kubefed
os::test::extended::focus "$@"

export OS_FEDERATION_NAME="${OS_FEDERATION_NAME:-origin-federation}"
export OS_FEDERATION_NAMESPACE="${OS_FEDERATION_NAMESPACE:-federation-system}"
export OS_FEDERATION_SERVICE_ACCOUNT="system:serviceaccount:${OS_FEDERATION_NAMESPACE}:default"

os::test::junit::declare_suite_start "extended/federation"

os::log::debug "Granting anyuid to the service account of the federation namespace to ensure /var/etcd is writeable in the etcd pod."
os::cmd::expect_success "oadm policy add-scc-to-user anyuid '${OS_FEDERATION_SERVICE_ACCOUNT}'"

os::log::info "Deploying federation control plane"
os::cmd::expect_success "kubefed init '${OS_FEDERATION_NAME}' --dns-provider=google-clouddns \
        --federation-system-namespace='${OS_FEDERATION_NAMESPACE}' \
        --etcd-persistent-storage=false

# Ensure the controller manager will be able to access cluster configuration stored as
# secrets in the federation namespace.
os::log::debug "Granting the admin role to the service account of the federation namespace"
os::cmd::expect_success "oadm --namespace '${OS_FEDERATION_NAMESPACE}' policy add-role-to-user admin '${OS_FEDERATION_SERVICE_ACCOUNT}'"

# TODO enable coredns when documentation is available
os::log::debug "Disabling the federation services controller to avoid having to configure dnsaas"
os::cmd::expect_success "oc --namespace='${OS_FEDERATION_NAMESPACE}' patch deploy '${OS_FEDERATION_NAME}-controller-manager' \
   --type=json -p='[{\"op\": \"add\", \"path\": \"/spec/template/spec/containers/0/command/-\", \"value\": \"--controllers=services=false\"}]'"

os::log::debug "Waiting for federation api to become available"
os::cmd::try_until_text "oc get --raw /healthz --config='${ADMIN_KUBECONFIG}' --context=${OS_FEDERATION_NAME}" 'ok' $(( 80 * second )) 0.25

os::log::info "Joining the host cluster to the federation"
export OS_FEDERATION_HOST_CONTEXT="$(oc config current-context)"
os::cmd::expect_success "kubefed join openshift --context='${OS_FEDERATION_NAME}' \
        --host-cluster-context='${OS_FEDERATION_HOST_CONTEXT}' \
        --cluster-context='${OS_FEDERATION_HOST_CONTEXT}'"

os::test::junit::declare_suite_end

os::log::info "Running federation tests"
FOCUS="Federation secrets.*successfully" SKIP="${SKIP_TESTS:-}" TEST_REPORT_FILE_NAME=federation os::test::extended::run -- -test.timeout 2h
