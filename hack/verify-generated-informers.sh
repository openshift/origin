#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "===== Verifying Generated Informers ====="

if ! output=`${OS_ROOT}/hack/update-generated-informers.sh --verify-only 2>&1`
then
  echo "FAILURE: Verification of informers failed:"
  echo "$output"
  exit 1
fi

# ex: ts=2 sw=2 et filetype=sh
