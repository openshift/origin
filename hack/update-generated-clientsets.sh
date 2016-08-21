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

os::build::os_version_vars
origin_version="v${OS_GIT_MAJOR}_${OS_GIT_MINOR%+}"

exit 0

packages=(
  github.com/openshift/origin/pkg/authorization
  github.com/openshift/origin/pkg/build
  github.com/openshift/origin/pkg/deploy
  github.com/openshift/origin/pkg/image
  github.com/openshift/origin/pkg/oauth
  github.com/openshift/origin/pkg/project
  github.com/openshift/origin/pkg/route
  github.com/openshift/origin/pkg/sdn
  github.com/openshift/origin/pkg/security
  github.com/openshift/origin/pkg/template
  github.com/openshift/origin/pkg/user
)


function generate_clientset_for() {
  local package="$1";shift
  local name="$1";shift
  pushd ${OS_ROOT} >/dev/null
  local common_args="--go-header-file=hack/boilerplate.txt"
  $clientgen --clientset-path="${package}/client/clientset_generated" \
             --input-base="${package}/api"                            \
             --output-base="../../.."                                 \
             --clientset-name="${name}"                               \
             $common_args                                             \
             "$@"
  popd >/dev/null
}

verify="${VERIFY:-}"

for pkg in "${packages[@]}"; do
  if [[ -z "${verify}" ]]; then
    # Remove deprecated/old files
    go list -f '{{.Dir}}' "${pkg}/client/clientset_generated/..." | xargs rm -rf
  fi
done

os::build::setup_env
for pkg in "${packages[@]}"; do
  generate_clientset_for "${pkg}" "internalclientset" --input=api/ "$@"
  generate_clientset_for "${pkg}" "release_${origin_version}" --input=api/v1 "$@"
done

