#!/bin/bash

set -e

hackdir=$(CDPATH="" cd $(dirname $0); pwd)

pushd ${hackdir}/../assets > /dev/null
  bundle exec grunt build
popd > /dev/null

pushd ${hackdir}/../ > /dev/null
  Godeps/_workspace/bin/go-bindata -prefix "assets/dist" -pkg "assets" -o "pkg/assets/bindata.go" assets/dist/...
popd > /dev/null