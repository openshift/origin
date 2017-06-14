#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "===== Verifying Generated Ugorji JSON Codecs ====="

if output=`find ${OS_ROOT}/vendor/k8s.io/kubernetes -name "types.generated.go" | grep -v "client-go"`
then
  echo "FAILURE: Verification of existing ugorji JSON codecs failed. These should NOT exist:"
  echo "$output"
  exit 1
fi

# ex: ts=2 sw=2 et filetype=sh
