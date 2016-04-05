#!/bin/bash
#
# This script verifies that our Bash scripts are written well by running the ShellCheck linter

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
cd "${OS_ROOT}"
source "${OS_ROOT}/hack/util.sh"
source "${OS_ROOT}/hack/cmd_util.sh"
os::log::install_errexit

if ! which shellcheck >&/dev/null; then
    echo "[ERROR] No \`shellcheck\` binary found, skipping linting..."
    exit 1
fi

files="$(find . -not \(                       \
             \(                               \
                 -path '*old-start-configs/*' \
                 -o -path '*Godeps/*'         \
             \) -prune                        \
         \) -name '*.sh')"

blacklist='
./test/end-to-end/core.sh
./test/cmd/diagnostics.sh
./test/cmd/edit.sh
./test/cmd/export.sh
./test/cmd/basicresources.sh
./test/cmd/images.sh
./test/cmd/router.sh
./test/cmd/debug.sh
./test/cmd/secrets.sh
./test/cmd/help.sh
./test/cmd/triggers.sh
./test/cmd/templates.sh
./test/cmd/policy.sh
./test/cmd/deployments.sh
./test/cmd/builds.sh
./test/cmd/admin.sh
./test/cmd/volumes.sh
./test/cmd/newapp.sh
./test/extended/ldap_groups.sh
./test/extended/fixtures/custom-secret-builder/build.sh
./test/extended/networking.sh
./test/extended/alternate_certs.sh
./test/extended/alternate_launches.sh
./test/extended/core.sh
./test/extended/cmd.sh
./test/extended/all.sh
./hack/test-end-to-end.sh
./hack/build-cross.sh
./hack/clean-assets.sh
./hack/copy-kube-artifacts.sh
./hack/rebase-kube.sh
./hack/build-release.sh
./hack/test-kube-e2e.sh
./hack/cmd_util.sh
./hack/gen-swagger-docs.sh
./hack/verify-jsonformat.sh
./hack/install-etcd.sh
./hack/release.sh
./hack/load-etcd-dump.sh
./hack/text.sh
./hack/verify-govet.sh
./hack/verify-shellcheck.sh
./hack/update-generated-completions.sh
./hack/verify-open-ports.sh
./hack/test-cmd.sh
./hack/export-certs.sh
./hack/test-go.sh
./hack/test-integration.sh
./hack/build-assets.sh
./hack/test-rpm.sh
./hack/test-tools.sh
./hack/verify-upstream-commits.sh
./hack/install-std-race.sh
./hack/extract-release.sh
./hack/build-in-docker.sh
./hack/make-p12-cert.sh
./hack/common.sh
./hack/pythia.sh
./hack/move-upstream.sh
./hack/update-generated-swagger-spec.sh
./hack/build-go.sh
./hack/rebase-describe-bumps.sh
./hack/test-cmd_util.sh
./hack/verify-generated-swagger-spec.sh
./hack/install-tools.sh
./hack/verify-gofmt.sh
./hack/update-generated-swagger-descriptions.sh
./hack/verify-generated-conversions.sh
./hack/convert-samples.sh
./hack/verify-generated-deep-copies.sh
./hack/cherry-pick.sh
./hack/dind-cluster.sh
./hack/verify-generated-completions.sh
./hack/update-generated-conversions.sh
./hack/verify-generated-docs.sh
./hack/update-generated-docs.sh
./hack/push-release.sh
./hack/update-generated-deep-copies.sh
./hack/install-assets.sh
./hack/lib/util/environment.sh
./hack/lib/log.sh
./hack/test-end-to-end-docker.sh
./hack/test-assets.sh
./hack/build-images.sh
./hack/serve-local-assets.sh
./hack/verify-generated-swagger-descriptions.sh
./hack/build-base-images.sh
./hack/update-external-examples.sh
./hack/verify-golint.sh
./hack/util.sh
./examples/zookeeper/teardown.sh
./examples/zookeeper/config-and-run.sh
./examples/project-spawner/project-spawner.sh
./examples/data-population/services.sh
./examples/data-population/quotas.sh
./examples/data-population/populate.sh
./examples/data-population/projects.sh
./examples/data-population/templates.sh
./examples/data-population/common.sh
./examples/data-population/apps.sh
./examples/data-population/users.sh
./examples/data-population/limits.sh
./examples/etcd/etcd-discovery.sh
./examples/etcd/etcd.sh
./examples/sample-app/pullimages.sh
./examples/sample-app/cleanup.sh
./images/node/scripts/origin-node-run.sh
./images/openldap/test-init.sh
./images/openldap/init.sh
./images/openvswitch/scripts/ovs-run.sh
./images/release/openshift-origin-build.sh
./images/ipfailover/keepalived/tests/verify_failover_image.sh
./images/ipfailover/keepalived/conf/settings.sh
./images/ipfailover/keepalived/monitor.sh
./images/ipfailover/keepalived/lib/utils.sh
./images/ipfailover/keepalived/lib/config-generators.sh
./images/ipfailover/keepalived/lib/failover-functions.sh
./images/builder/docker/custom-docker-builder/build.sh
./contrib/vagrant/provision-config.sh
./contrib/vagrant/provision-node.sh
./contrib/vagrant/provision-util.sh
./contrib/vagrant/provision-master.sh
./contrib/vagrant/provision-minimal.sh
./contrib/vagrant/provision-full.sh
./contrib/vagrant/provision-dind.sh
./contrib/node/install-sdn.sh
./tools/junitreport/test/integration.sh
'
for file in ${files}; do
    if ! echo "${blacklist}" | grep -q "${file}"; then
        if ! shellcheck "${file}"; then
            failed=true
        fi
    fi
done

if [[ "${failed:-}" = "true" ]]; then
    echo "[FAILURE] ShellCheck linting on shell scripts failed!"
    exit 1
else 
    echo "[SUCCESS] ShellCheck linting on shell scripts succeeded!"
    exit 0
fi