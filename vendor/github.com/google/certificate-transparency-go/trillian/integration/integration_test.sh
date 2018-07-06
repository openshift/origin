#!/bin/bash
set -e
. "${GOPATH}"/src/github.com/google/trillian/integration/functions.sh

run_test "CT integration test" "${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/integration/ct_integration_test.sh" 1 1 1
run_test "CT multi-server integration test" "${GOPATH}/src/github.com/google/certificate-transparency-go/trillian/integration/ct_integration_test.sh" 3 3 3
