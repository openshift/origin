#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

if [[ "${PROTO_OPTIONAL:-}" == "1" ]]; then
  os::log::warning "Skipping protobuf generation as \$PROTO_OPTIONAL is set."
  exit 0
fi

os::util::ensure::system_binary_exists 'protoc'
if [[ "$(protoc --version)" != "libprotoc 3.0."* ]]; then
  os::log::fatal "Generating protobuf requires protoc 3.0.x. Please download and
install the platform appropriate Protobuf package for your OS:

  https://github.com/google/protobuf/releases

To skip protobuf generation, set \$PROTO_OPTIONAL."
fi

os::util::ensure::gopath_binary_exists 'goimports'
os::build::setup_env

os::util::ensure::built_binary_exists 'genprotobuf'
os::util::ensure::built_binary_exists 'protoc-gen-gogo' vendor/k8s.io/kubernetes/cmd/libs/go2idl/go-to-protobuf/protoc-gen-gogo

genprotobuf --output-base="${GOPATH}/src" "$@"
