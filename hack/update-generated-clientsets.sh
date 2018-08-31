#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

os::util::ensure::built_binary_exists 'client-gen' 'vendor/k8s.io/kubernetes/staging/src/k8s.io/code-generator/cmd/client-gen'

# list of package to generate client set for
packages=(
  github.com/openshift/origin/pkg/authorization/apis/authorization
  github.com/openshift/origin/pkg/image/apis/image
  github.com/openshift/origin/pkg/oauth/apis/oauth
  github.com/openshift/origin/pkg/project/apis/project
  github.com/openshift/origin/pkg/quota/apis/quota
  github.com/openshift/origin/pkg/route/apis/route
  github.com/openshift/origin/pkg/network/apis/network
  github.com/openshift/origin/pkg/security/apis/security
  github.com/openshift/origin/pkg/template/apis/template
  github.com/openshift/origin/pkg/user/apis/user
)

function generate_clientset_for() {
  local package="$1";shift
  local name="$1";shift
  echo "-- Generating ${name} client set for ${package} ..."
  grouppkg=$(realpath --canonicalize-missing --relative-to=$(pwd) ${package}/..)
  client-gen --clientset-path="${grouppkg}/generated" \
             --input-base="${package}"                            \
             --output-base="../../.."                           \
             --clientset-name="${name}"                               \
             --go-header-file=hack/boilerplate.txt                    \
             "$@"
}

verify="${VERIFY:-}"

# remove the old client sets if we're not verifying
if [[ -z "${verify}" ]]; then
  for pkg in "${packages[@]}"; do
    grouppkg=$(realpath --canonicalize-missing --relative-to=$(pwd) ${pkg}/../..)
    # delete all generated go files excluding files named *_expansion.go
    go list -f '{{.Dir}}' "${grouppkg}/generated/internalclientset" \
		| xargs -n1 -I{} find {} -type f -not -name "*_expansion.go" -delete
  done
fi

for pkg in "${packages[@]}"; do
  shortGroup=$(basename "${pkg}")
  containingPackage=$(dirname "${pkg}")
  generate_clientset_for "${containingPackage}" "internalclientset"  --input=${shortGroup} ${verify} "$@"
done

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../../../k8s.io/code-generator)}

${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,lister,informer" \
  github.com/openshift/origin/pkg/cmd/openshift-operators/generated \
  github.com/openshift/origin/pkg/cmd/openshift-operators/apis \
  "webconsole:v1alpha1" \
  --go-header-file ${SCRIPT_ROOT}/hack/boilerplate.txt \
  ${verify}
