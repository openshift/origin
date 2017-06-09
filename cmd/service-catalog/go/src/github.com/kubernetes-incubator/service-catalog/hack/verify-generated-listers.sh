#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "===== Verifying Generated Listers ====="

if ! output=`${OS_ROOT}/hack/update-generated-listers.sh --verify-only 2>&1`
then
  echo "FAILURE: Verification of listers failed:"
  echo "$output"
  exit 1
fi

# ex: ts=2 sw=2 et filetype=sh
