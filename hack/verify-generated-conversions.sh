#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "===== Verifying Generated Conversions ====="

if ! output=`${OS_ROOT}/hack/update-generated-conversions.sh --verify-only 2>&1`
then
  echo "FAILURE: Verification of conversions failed:"
  echo "$output"
  exit 1
fi

# ex: ts=2 sw=2 et filetype=sh
