#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..

if [[ "$(protoc --version)" != "libprotoc 3.0."* ]]; then
  echo "Generating protobuf requires protoc 3.0.x. Please download and
install the platform appropriate Protobuf package for your OS:

  https://github.com/google/protobuf/releases

To skip protobuf generation, set \$PROTO_OPTIONAL."
fi

rm -rf go-to-protobuf
go build -o _output/bin/go-to-protobuf github.com/openshift/api/vendor/k8s.io/code-generator/cmd/go-to-protobuf

go-to-protobuf \
--output-base="${GOPATH}/src" \
--apimachinery-packages='-k8s.io/apimachinery/pkg/util/intstr,-k8s.io/apimachinery/pkg/api/resource,-k8s.io/apimachinery/pkg/runtime/schema,-k8s.io/apimachinery/pkg/runtime,-k8s.io/apimachinery/pkg/apis/meta/v1,-k8s.io/apimachinery/pkg/apis/meta/v1alpha1,-k8s.io/api/core/v1' \
--go-header-file=${SCRIPT_ROOT}/hack/empty.txt \
--proto-import=${SCRIPT_ROOT}/vendor/github.com/gogo/protobuf/protobuf \
--proto-import=${SCRIPT_ROOT}/vendor \
--packages='github.com/openshift/api/apps/v1,github.com/openshift/api/authorization/v1,github.com/openshift/api/image/v1,github.com/openshift/api/network/v1,github.com/openshift/api/oauth/v1,github.com/openshift/api/project/v1,github.com/openshift/api/quota/v1,github.com/openshift/api/route/v1,github.com/openshift/api/security/v1,github.com/openshift/api/template/v1,github.com/openshift/api/user/v1'
