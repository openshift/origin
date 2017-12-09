#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

os::util::ensure::built_binary_exists 'lister-gen' 'vendor/k8s.io/kubernetes/staging/src/k8s.io/code-generator/cmd/lister-gen'

# list of package to generate listers for
packages=(
  github.com/openshift/origin/pkg/authorization/apis/authorization
  github.com/openshift/origin/pkg/build/apis/build
  github.com/openshift/origin/pkg/apps/apis/apps
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

function generate_listers_for() {
  local package="$1";shift
  echo "-- Generating listers for ${package} ..."
  grouppkg=$(realpath --canonicalize-missing --relative-to=$(pwd) ${package}/../..)
  lister-gen --logtostderr \
             --go-header-file=hack/boilerplate.txt \
             --input-dirs="${package}" \
             --output-package="${grouppkg}/generated/listers" \
             "$@"
}

verify="${VERIFY:-}"

# remove the old listers
# doing this deletes the custom indexes which breaks templates.  I don't know how templates worked before
for pkg in "${packages[@]}"; do
  if [[ -z "${verify}" ]]; then
    grouppkg=$(realpath --canonicalize-missing --relative-to=$(pwd) ${pkg}/../..)
    # go list -f '{{.Dir}}' "${grouppkg}/generated/listers/..." | xargs rm -rf
  fi
done

for pkg in "${packages[@]}"; do
  generate_listers_for "${pkg}"
done
