#!/bin/bash

set -o nounset
set -o pipefail

GO_VERSION=($(go version))

if [[ -z $(echo "${GO_VERSION[2]}" | grep -E 'go1.4?(\.[0-9]+)') ]]; then
  echo "Unknown go version '${GO_VERSION[2]}', skipping go vet."
  exit 0
fi

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"

cd "${OS_ROOT}"
mkdir -p _output/govet

FAILURE=false
test_dirs=$(find_files | cut --delimiter=/ --fields=1-2 | sort -u)
for test_dir in $test_dirs
do
  go tool vet -printf=false \
              -methods=false \
              -composites=false \
              -shadow=false \
              $test_dir 2>&1 | sed '/exit status/d'
  if [ "$?" -ne 0 ]
  then 
    FAILURE=true
  fi
done

# We don't want to exit on the first failure of go vet, so just keep track of 
# whether a failure occured or not.
if $FAILURE
then
  echo "FAILURE: go vet failed!"
  exit 1
else
  echo "SUCCESS: go vet succeded!"
  exit 0
fi