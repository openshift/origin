#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "===== Verifying Generated Deep Copies ====="

if ! output=`${OS_ROOT}/hack/update-generated-deep-copies.sh --verify-only 2>&1`
then
  echo "FAILURE: Verifying deep copies failed:"
  echo "$output"
  exit 1
fi

# ex: ts=2 sw=2 et filetype=sh
