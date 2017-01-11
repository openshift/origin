#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "===== Verifying Generated OpenAPI ====="

if ! output=`${OS_ROOT}/hack/update-generated-openapi.sh --verify-only 2>&1`
then
  echo "FAILURE: Verification of openapi failed:"
  echo "$output"
  exit 1
fi

# ex: ts=2 sw=2 et filetype=sh
