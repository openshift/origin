#!/bin/sh -eu

builtins='[a-z0-9]+|struct{}'
pkgs='k8s\.io/api/.*|k8s\.io/apimachinery/.*|github\.com/openshift/api/.*'

# Check that types used in OpenShift API are designed to be used in API.
#
# For example, time.Duration does not implement encoding/json.Unmarshaler, so
# it is encoded as integer nanoseconds, which can be inconvenient. There is the
# wrapper k8s.io/apimachinery/pkg/apis/meta/v1.Duration that marshals into
# strings, and it should be preferred to time.Duration.
#
# The whitelisted types are
#
#   * go built-in types
#   * types that are defined in this package (FooSpec, FooStatus, etc.)
#   * types from k8s.io API packages
#
go run ./hack/typelinter \
    -whitelist="^(?:\[]|\*|map\[string])*(?:$builtins|(?:$pkgs)\.[A-Za-z0-9]+)\$" \
    -excluded=github.com/openshift/api/build/v1.BuildStatus:Duration \
    -excluded=github.com/openshift/api/image/dockerpre012.Config:ExposedPorts \
    -excluded=github.com/openshift/api/image/dockerpre012.ImagePre012:Created \
    -excluded=github.com/openshift/api/imageregistry/v1.ImagePrunerSpec:KeepYoungerThan \
    ./...
