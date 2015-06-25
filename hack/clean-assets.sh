#!/bin/bash

set -e

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

pushd ${OS_ROOT}/assets > /dev/null
  echo "Cleaning up bower_components, node_modules, and dist directories..."
  rm -rf bower_components/*
  rm -rf node_modules/*
  rm -rf dist/*
  rm -rf dist.*/*

  if which bower > /dev/null 2>&1 ; then
    # In case upstream components change things without incrementing versions
    echo "Clearing bower cache..."
    bower cache clean --allow-root
  else
    echo "Skipping bower cache clean, bower not installed."
  fi
popd > /dev/null