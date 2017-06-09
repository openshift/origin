#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

os::util::ensure::built_binary_exists 'geninformers'

# list of package to generate informers for
packages=(
  github.com/openshift/origin/pkg/authorization
  github.com/openshift/origin/pkg/build
  github.com/openshift/origin/pkg/deploy
  github.com/openshift/origin/pkg/image
  github.com/openshift/origin/pkg/oauth
  github.com/openshift/origin/pkg/project
  github.com/openshift/origin/pkg/quota
  github.com/openshift/origin/pkg/route
  github.com/openshift/origin/pkg/sdn
  github.com/openshift/origin/pkg/template
  github.com/openshift/origin/pkg/user
)

function generate_informers_for() {
  local package="$1";shift
  echo "-- Generating informers for ${package} ..."
  geninformers --logtostderr \
               --go-header-file=hack/boilerplate.txt \
               --input-dirs="${package}/api,${package}/api/v1" \
               --output-package="${package}/generated/informers" \
               --versioned-clientset-package "${package}/generated/clientset" \
               --internal-clientset-package "${package}/generated/internalclientset" \
               --listers-package "${package}/generated/listers" \
               "$@"
}

verify="${VERIFY:-}"

# remove the old informers
for pkg in "${packages[@]}"; do
  if [[ -z "${verify}" ]]; then
    go list -f '{{.Dir}}' "${pkg}/generated/informers/..." | xargs rm -rf
  fi
done

for pkg in "${packages[@]}"; do
  generate_informers_for "${pkg}"
done
