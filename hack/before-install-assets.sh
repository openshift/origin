#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# If we are running inside of Travis then do not run the rest of this
# script unless we want to TEST_ASSETS
if [[ "${TRAVIS-}" == "true" && "${TEST_ASSETS-}" == "false" ]]; then
  exit
fi

sudo apt-get install -qq ruby