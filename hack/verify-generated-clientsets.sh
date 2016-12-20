#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "===== Verifying Generated Client sets ====="

if output=$(VERIFY=true ${OS_ROOT}/hack/update-generated-clientsets.sh 2>&1); then
  echo "SUCCESS: Generated client sets up to date."
else
  echo "${output}"
  echo "FAILURE: Generated client sets out of date. Please run hack/update-generated-clientsets.sh"
  exit 1
fi
