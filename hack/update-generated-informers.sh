#!/usr/bin/env bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

os::util::ensure::built_binary_exists 'informer-gen' 'vendor/k8s.io/kubernetes/staging/src/k8s.io/code-generator/cmd/informer-gen'

# list of package to generate informers for
packages=(
  github.com/openshift/origin/pkg/quota/apis/quota
)

function generate_informers_for() {
  local package="$1";shift
  echo "-- Generating informers for ${package} ..."
  grouppkg=$(realpath --canonicalize-missing --relative-to=$(pwd) ${package}/../..)
  informer-gen --logtostderr \
               --go-header-file=hack/boilerplate.txt \
               --input-dirs="${package}" \
               --output-package="${grouppkg}/generated/informers" \
               --internal-clientset-package "${grouppkg}/generated/internalclientset" \
               --listers-package "${grouppkg}/generated/listers" \
               "$@"
}

verify="${VERIFY:-}"

# remove the old informers
for pkg in "${packages[@]}"; do
  if [[ -z "${verify}" ]]; then
    grouppkg=$(realpath --canonicalize-missing --relative-to=$(pwd) ${pkg}/../..)
    go list -f '{{.Dir}}' "${grouppkg}/generated/informers/..." | xargs rm -rf
  fi
done

for pkg in "${packages[@]}"; do
  generate_informers_for "${pkg}"
done
