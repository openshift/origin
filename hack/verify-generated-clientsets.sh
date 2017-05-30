#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "===== Verifying Generated Client sets ====="
if output=$(VERIFY=--verify-only ${OS_ROOT}/hack/update-generated-clientsets.sh 2>&1)
then
  echo "SUCCESS: Generated client sets up to date."
else
  echo "FAILURE: Generated client sets out of date. Please run hack/update-generated-clientsets.sh"
  echo "${output}"
  exit 1
fi
