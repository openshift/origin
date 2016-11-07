#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

if [[ -z "$(which protoc)" || "$(protoc --version)" != "libprotoc 3.0."* ]]; then
  echo "Generating protobuf requires protoc 3.0.x. Please download and"
  echo "install the platform appropriate Protobuf package for your OS: "
  echo
  echo "  https://github.com/google/protobuf/releases"
  echo
  if [[ "${PROTO_OPTIONAL:-}" == "1" ]]; then
    exit 0
  fi
  exit 1
fi

if [[ -z "$(which goimports)" ]]; then
  echo "goimports is required to be present on your path in order to format the generated"
  echo "protobuf files"
  echo
  exit 1
fi

os::build::setup_env

os::util::ensure::built_binary_exists 'genprotobuf'
os::util::ensure::built_binary_exists 'protoc-gen-gogo' vendor/k8s.io/kubernetes/cmd/libs/go2idl/go-to-protobuf/protoc-gen-gogo

PATH="$( dirname "${genprotobuf}" ):${PATH}" ${genprotobuf} --output-base="${GOPATH}/src" "$@"
