#!/bin/bash

set -o nounset
set -o pipefail

GO_VERSION=($(go version))

if [[ -z $(echo "${GO_VERSION[2]}" | grep -E 'go1.5') ]]; then
  echo "Unknown go version '${GO_VERSION}', skipping go vet."
  exit 0
fi

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"

cd "${OS_ROOT}"
mkdir -p _output/govet

os::build::setup_env

FAILURE=false
test_dirs=$(find_files | cut -d '/' -f 1-2 | sort -u)
for test_dir in $test_dirs
do
  go tool vet -all $test_dir
  if [ "$?" -ne 0 ]
  then
    FAILURE=true
  fi
done

# For the sake of slowly white-listing `shadow` checks, we need to keep track of which
# directories we're searching through. The following are all of the directories we care about:
ALL_DIRS='./cmd
./examples
./images
./pkg/api
./pkg/assets
./pkg/auth
./pkg/authorization
./pkg/build
./pkg/client
./pkg/cmd
./pkg/config
./pkg/controller
./pkg/deploy
./pkg/diagnostics
./pkg/dns
./pkg/dockerregistry
./pkg/generate
./pkg/gitserver
./pkg/image
./pkg/ipfailover
./pkg/oauth
./pkg/project
./pkg/route
./pkg/router
./pkg/sdn
./pkg/security
./pkg/service
./pkg/serviceaccounts
./pkg/template
./pkg/user
./pkg/util
./pkg/version
./plugins
./test'

# Whitelist some directories using a `grep`
SHADOW_TEST_DIRS=$(echo "${ALL_DIRS}" | grep -E "\./(test)")
for test_dir in $SHADOW_TEST_DIRS
do
  go tool vet -shadow -shadowstrict $test_dir
  if [ "$?" -ne 0 ]
  then
    FAILURE=true
  fi
done

# We don't want to exit on the first failure of go vet, so just keep track of
# whether a failure occurred or not.
if $FAILURE
then
  echo "FAILURE: go vet failed!"
  exit 1
else
  echo "SUCCESS: go vet succeded!"
  exit 0
fi
