#!/bin/bash
set -x
pushd `dirname $0`/../../..
hack/build-go.sh cmd/openshift-router
cp _output/go/bin/openshift-router images/router/haproxy/bin/
