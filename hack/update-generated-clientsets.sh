#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

"${OS_ROOT}/hack/build-go.sh" vendor/k8s.io/kubernetes/cmd/libs/go2idl/client-gen

# Find binary
clientgen="$(os::build::find-binary client-gen)"

if [[ ! "$clientgen" ]]; then
  {
    echo "It looks as if you don't have a compiled client-gen binary"
    echo
    echo "If you are running from a clone of the git repo, please run"
    echo "'./hack/build-go.sh vendor/k8s.io/kubernetes/cmd/libs/go2idl/client-gen'."
  } >&2
  exit 1
fi

# list of package to generate client set for
packages=(
  github.com/openshift/origin/pkg/authorization
  github.com/openshift/origin/pkg/build
  github.com/openshift/origin/pkg/deploy
  github.com/openshift/origin/pkg/image
  github.com/openshift/origin/pkg/oauth
  github.com/openshift/origin/pkg/project
  github.com/openshift/origin/pkg/route
  github.com/openshift/origin/pkg/sdn
  github.com/openshift/origin/pkg/template
  github.com/openshift/origin/pkg/user
)

function generate_clientset_for() {
  local package="$1";shift
  local name="$1";shift
  echo "-- Generating ${name} client set for ${package} ..."
  $clientgen --clientset-path="${package}/client/clientset_generated" \
             --clientset-api-path="/oapi"                             \
             --input-base="${package}/api"                            \
             --output-base="../../.."                                 \
             --clientset-name="${name}"                               \
             --go-header-file=hack/boilerplate.txt                    \
             "$@"
}

verify="${VERIFY:-}"

# remove the old client sets
for pkg in "${packages[@]}"; do
  if [[ -z "${verify}" ]]; then
    go list -f '{{.Dir}}' "${pkg}/client/clientset_generated/..." | xargs rm -rf
  fi
done

# get the tag name for the current origin release
os::build::get_version_vars
origin_version="v${OS_GIT_MAJOR}_${OS_GIT_MINOR%+}"

for pkg in "${packages[@]}"; do
  generate_clientset_for "${pkg}" "internalclientset" --input=api/ "$@"
  generate_clientset_for "${pkg}" "release_${origin_version}" --input=api/v1 "$@"
done

