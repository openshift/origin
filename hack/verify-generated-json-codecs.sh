#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "===== Verifying Generated Ugorji JSON Codecs ====="

output=$(find ${OS_ROOT}/vendor/k8s.io/kubernetes -name "types.generated.go")
if [[ -n "${output}" ]]; then
  os::log::fatal "FAILURE: Verification of existing ugorji JSON codecs failed. These should NOT exist:\n${output}"
fi
