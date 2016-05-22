#!/bin/bash

set -o nounset
set -o pipefail

GO_VERSION=($(go version))

if [[ -z $(echo "${GO_VERSION[2]}" | grep -E 'go1.[6]') && -z "${FORCE_VERIFY-}" ]]; then
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
  go tool vet -shadow=false $test_dir
  if [ "$?" -ne 0 ]
  then
    FAILURE=true
  fi
done

# For the sake of slowly white-listing `shadow` checks, we need to keep track of which
# directories we're searching through. The following are all of the directories we care about:
# all top-level directories except for 'pkg', and all second-level subdirectories of 'pkg'.
ALL_DIRS=$(find_files | grep -Eo "\./([^/]+|pkg/[^/]+)" | sort -u)

DIR_BLACKLIST='./hack
./pkg/api
./pkg/authorization
./pkg/build
./pkg/client
./pkg/cmd
./pkg/deploy
./pkg/diagnostics
./pkg/dockerregistry
./pkg/generate
./pkg/gitserver
./pkg/image
./pkg/oauth
./pkg/project
./pkg/router
./pkg/security
./pkg/serviceaccounts
./pkg/template
./pkg/user
./pkg/util
./test
./third_party
./tools'

for test_dir in $ALL_DIRS
do
  # use `grep` failure to determine that a directory is not in the blacklist
  if ! echo "${DIR_BLACKLIST}" | grep -q "${test_dir}"; then
    go tool vet -shadow -shadowstrict $test_dir
    if [ "$?" -ne "0" ]
    then
      FAILURE=true
    fi
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
