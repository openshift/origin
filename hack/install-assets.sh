#!/bin/bash

set -e

# If we are running inside of Travis then do not run the rest of this
# script unless we want to TEST_ASSETS
if [[ "${TRAVIS}" == "true" && "${TEST_ASSETS}" == "false" ]]; then
  exit
fi

npm install -g bower grunt-cli
pushd assets > /dev/null
  npm install
  bower install
popd > /dev/null
gem install compass