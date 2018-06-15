#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

echo $(go version)

S2I_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${S2I_ROOT}/hack/util.sh"

cd "${S2I_ROOT}"

FAILURE=false
test_dirs=$(s2i::util::find_files | cut -d '/' -f 1-2 | sort -u)
for test_dir in $test_dirs
do
  if ! go tool vet -shadow=false -composites=false $test_dir
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
  echo "SUCCESS: go vet succeeded!"
  exit 0
fi
