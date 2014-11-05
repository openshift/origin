#!/bin/bash

set -e

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# If we are running inside of Travis then do not run the rest of this
# script unless we want to TEST_ASSETS
if [[ "${TRAVIS}" == "true" && "${TEST_ASSETS}" == "false" ]]; then
  exit
fi

# Lock version of npm to work around https://github.com/npm/npm/issues/6309
if [[ "${TRAVIS}" == "true" ]]; then
  npm install npm@2.1.1 -g
fi

# Install bower if needed
if ! which bower > /dev/null 2>&1 ; then
  if [[ "${TRAVIS}" == "true" ]]; then
    npm install -g bower
  else
    sudo npm install -g bower
  fi
fi
 
# Install grunt if needed
if ! which grunt > /dev/null 2>&1 ; then
  if [[ "${TRAVIS}" == "true" ]]; then
    npm install -g grunt-cli
  else
    sudo npm install -g grunt-cli
  fi
fi

pushd ${OS_ROOT}/assets > /dev/null
  npm install

  # In case upstream components change things without incrementing versions
  bower cache clean
  bower install

  bundle install --path ${OS_ROOT}/assets/.bundle
popd > /dev/null

pushd ${OS_ROOT}/Godeps/_workspace > /dev/null
  godep_path=$(pwd)
  pushd src/github.com/jteeuwen/go-bindata > /dev/null
    GOPATH=$godep_path go install ./...
  popd > /dev/null
popd > /dev/null
