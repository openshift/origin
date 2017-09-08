#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

os::util::ensure::built_binary_exists 'client-gen' 'vendor/k8s.io/kubernetes/cmd/libs/go2idl/client-gen'

# list of package to generate client set for
packages=(
  github.com/openshift/origin/pkg/authorization
  github.com/openshift/origin/pkg/build
  github.com/openshift/origin/pkg/apps
  github.com/openshift/origin/pkg/image
  github.com/openshift/origin/pkg/oauth
  github.com/openshift/origin/pkg/project
  github.com/openshift/origin/pkg/quota
  github.com/openshift/origin/pkg/route
  github.com/openshift/origin/pkg/sdn
  github.com/openshift/origin/pkg/template
  github.com/openshift/origin/pkg/user
)

function generate_clientset_for() {
  local package="$1";shift
  local name="$1";shift
  echo "-- Generating ${name} client set for ${package} ..."
  client-gen --clientset-path="${package}/generated" \
             --input-base="${package}"                            \
             --output-base="../../.."                                 \
             --clientset-name="${name}"                               \
             --go-header-file=hack/boilerplate.txt                    \
             "$@"
}

verify="${VERIFY:-}"

# remove the old client sets
for pkg in "${packages[@]}"; do
  if [[ -z "${verify}" ]]; then
    go list -f '{{.Dir}}' "${pkg}/generated/clientset/..." "${pkg}/generated/internalclientset/..." | xargs rm -rf
  fi
done

for pkg in "${packages[@]}"; do
  shortGroup=$(basename "${pkg}")
  generate_clientset_for "${pkg}" "internalclientset"  --group=${shortGroup} --input=api/ "$@"
  generate_clientset_for "${pkg}" "clientset" --group=${shortGroup} --version=v1 --input=api/v1 "$@"
done
