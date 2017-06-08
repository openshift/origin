#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

os::util::ensure::built_binary_exists 'genlisters'

# list of package to generate listers for
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

function generate_listers_for() {
  local package="$1";shift
  echo "-- Generating listers for ${package} ..."
  genlisters --logtostderr \
             --go-header-file=hack/boilerplate.txt \
             --input-dirs="${package}/api,${package}/api/v1" \
             --output-package="${package}/generated/listers" \
             "$@"
}

verify="${VERIFY:-}"

# remove the old listers
for pkg in "${packages[@]}"; do
  if [[ -z "${verify}" ]]; then
    go list -f '{{.Dir}}' "${pkg}/generated/listers/..." | xargs rm -rf
  fi
done

for pkg in "${packages[@]}"; do
  generate_listers_for "${pkg}"
done
