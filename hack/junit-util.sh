#!/bin/bash

# junit-util builds the jUnit report parsing bianries and sets up the output directories for 
# test output as well as finished jUnit XML. the XML will be copied to `artifacts/junit` on Jenkins

set -o errexit
set -o nounset
set -o pipefail

STARTTIME=$(date +%s)
OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

platform="$(os::build::host_platform)"
"${OS_ROOT}/hack/build-go.sh" Godeps/_workspace/src/github.com/jstemmer/go-junit-report

# Find binary
junitreport=$( (ls -t _output/local/bin/${platform}/go-junit-report) 2>/dev/null || true | head -1 )

if [[ ! "$junitreport" ]]; then
  {
    echo "It looks as if you don't have a compiled go-junit-report binary"
    echo
    echo "If you are running from a clone of the git repo, please run"
    echo "'./hack/build-go.sh Godeps/_workspace/src/github.com/jstemmer/go-junit-report'."
  } >&2
  exit 1
fi

# TODO(skuznets): add the internal cmd/junitreport binary build and check here for test-cmd 

export JUNIT_OUTPUT_DIR="/tmp/openshift/junit/output"
export JUNIT_REPORT_DIR="/tmp/openshift/junit/report"
mkdir -p "${JUNIT_REPORT_DIR}" 
mkdir -p "${JUNIT_OUTPUT_DIR}"
