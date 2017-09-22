#!/bin/bash
#
# Runs the Kubernetes conformance suite against an OpenShift cluster
#
# Test prerequisites:
#
# * all nodes that users can run workloads under marked as schedulable
#
source "$(dirname "${BASH_SOURCE}")/../../hack/lib/init.sh"

# Check inputs
if [[ -z "${KUBECONFIG-}" ]]; then
  os::log::fatal "KUBECONFIG must be set to a root account"
fi
test_report_dir="${ARTIFACT_DIR}"
mkdir -p "${test_report_dir}"

version="${KUBERNETES_VERSION:-v1.7.6}"
kubernetes="${KUBERNETES_ROOT:-${OS_ROOT}/../../../k8s.io/kubernetes}"
if [[ ! -d "${kubernetes}" ]]; then
  if [[ -n "${KUBERNETES_ROOT-}" ]]; then
    os::log::fatal "Cannot find Kubernetes source directory, set KUBERNETES_ROOT"
  fi
  kubernetes="${OS_ROOT}/_output/components/kubernetes"
  mkdir -p "$( dirname "${kubernetes}" )"
  os::log::info "Cloning Kubernetes source"
  git clone "https://github.com/kubernetes/kubernetes.git" -b "${version}" --depth=1 "${kubernetes}"
fi

os::log::info "Running Kubernetes conformance suite for ${version}"

# Execute OpenShift prerequisites
# Disable container security
oc adm policy add-scc-to-group privileged system:authenticated system:serviceaccounts
oc adm policy remove-scc-from-group restricted system:authenticated
oc adm policy remove-scc-from-group anyuid system:cluster-admins
# Mark the masters and infra nodes as unschedulable so tests ignore them
oc get nodes -o name -l 'role in (infra,master)' | xargs -L1 oc adm cordon
unschedulable="$( oc get nodes -o name -l 'role in (infra,master)' | wc -l )"
# TODO: undo these operations

# Execute Kubernetes prerequisites
pushd "${kubernetes}" > /dev/null
git checkout "${version}"
make WHAT=cmd/kubectl
make WHAT=test/e2e/e2e.test
export PATH="${kubernetes}/_output/local/bin/$( os::build::host_platform ):${PATH}"
kubectl version

# Run the test
e2e.test '-ginkgo.focus=\[Conformance\]' \
  -report-dir "${test_report_dir}" -ginkgo.noColor \
  -allowed-not-ready-nodes ${unschedulable} \
  2>&1 | tee "${test_report_dir}/e2e.log"