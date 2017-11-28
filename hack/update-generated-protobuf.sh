#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

if [[ "${PROTO_OPTIONAL:-}" == "1" ]]; then
  os::log::warning "Skipping protobuf generation as \$PROTO_OPTIONAL is set."
  exit 0
fi

os::util::ensure::system_binary_exists 'protoc'
if [[ "$(protoc --version)" != "libprotoc 3."* ]]; then
  os::log::fatal "Generating protobuf requires protoc 3. Please download and
install the platform appropriate Protobuf package for your OS:

  https://github.com/google/protobuf/releases

To skip protobuf generation, set \$PROTO_OPTIONAL."
fi

os::util::ensure::gopath_binary_exists 'goimports'
os::build::setup_env

os::util::ensure::built_binary_exists 'go-to-protobuf' vendor/k8s.io/code-generator/cmd/go-to-protobuf
os::util::ensure::built_binary_exists 'protoc-gen-gogo' vendor/k8s.io/code-generator/cmd/go-to-protobuf/protoc-gen-gogo

# references to k8s packages for import, but not generation
APIMACHINERY_PACKAGES=(
  -k8s.io/api/core/v1
  -k8s.io/apimachinery/pkg/util/intstr
  -k8s.io/apimachinery/pkg/api/resource
  -k8s.io/apimachinery/pkg/runtime/schema
  -k8s.io/apimachinery/pkg/runtime
  -k8s.io/apimachinery/pkg/apis/meta/v1
  -k8s.io/apimachinery/pkg/apis/meta/v1alpha1
)

# references to openshift packages for generation
PACKAGES=(
  github.com/openshift/origin/pkg/authorization/apis/authorization/v1
  github.com/openshift/origin/pkg/build/apis/build/v1
  github.com/openshift/origin/pkg/apps/apis/apps/v1
  github.com/openshift/origin/pkg/image/apis/image/v1
  github.com/openshift/origin/pkg/oauth/apis/oauth/v1
  github.com/openshift/origin/pkg/project/apis/project/v1
  github.com/openshift/origin/pkg/quota/apis/quota/v1
  github.com/openshift/origin/pkg/route/apis/route/v1
  github.com/openshift/origin/pkg/network/apis/network/v1
  github.com/openshift/origin/pkg/security/apis/security/v1
  github.com/openshift/origin/pkg/template/apis/template/v1
  github.com/openshift/origin/pkg/user/apis/user/v1

)

# requires the 'proto' tag to build (will remove when ready)
# searches for the protoc-gen-gogo extension in the output directory
# satisfies import of github.com/gogo/protobuf/gogoproto/gogo.proto and the
# core Google protobuf types
go-to-protobuf \
  --go-header-file="${OS_ROOT}/hack/boilerplate.txt" \
  --proto-import="${OS_ROOT}/vendor" \
  --proto-import="${OS_ROOT}/vendor/k8s.io/kubernetes/third_party/protobuf" \
  --packages=$(IFS=, ; echo "${PACKAGES[*]}") \
  --apimachinery-packages=$(IFS=, ; echo "${APIMACHINERY_PACKAGES[*]}") \
  "$@"
