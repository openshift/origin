#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

run_individual_tests() {
  files_to_test=$1
  tests="printf methods structtags composites asmdecl assign atomic bool buildtags copylocks nilfunc rangeloops unreachable shadow unsafeptr unusedresult"

  for test in $tests
  do
    touch "_output/govet/${test}.log"
    for file in $files_to_test
    do
      go tool vet "-${test}=true" $file 2>&1 | sed '/exit status/d' >> "_output/govet/${test}.log" || true
    done
    if [ -s "_output/govet/${test}.log" ]
    then
      echo "FAILURE: go vet -${test} had errors: "
      cat "_output/govet/${test}.log"
    else 
      echo "SUCCESS: go vet -${test} had no errors"
    fi
  done
}

GO_VERSION=($(go version))

if [[ -z $(echo "${GO_VERSION[2]}" | grep -E 'go1.4') ]]; then
  echo "Unknown go version '${GO_VERSION}', skipping go vet."
  exit 0
fi

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"

cd "${OS_ROOT}"
mkdir -p _output/govet

files=$(find_files)
# Run once all together to see if errors exist
all_log="_output/govet/all.log"
touch $all_log
for file in $files
    do
      go tool vet -all $file 2>&1 | sed '/exit status/d' >> $all_log || true
    done
    if [ -s "${all_log}" ]
    then
      # If they do, re-run with each individual test to pick up as many errors as possible
      run_individual_tests "${files}"
      rm -rf _output/govet
      exit 1
    else 
      echo "SUCCESS: go vet had no errors"
      rm -rf _output/govet
      exit 0
fi

