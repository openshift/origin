#!/bin/bash

# This script installs std -race on Travis (see https://code.google.com/p/go/issues/detail?id=6479)

set -e

if [ "${TRAVIS}" == "true" ]; then
  GO_VERSION=($(go version))

  if [ ${GO_VERSION[2]} \< "go1.3" ]; then
    echo "Installing the -race compatible version of the std go library"
    go install -a -race std
  fi
fi
