#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "===== Verifying Generated Client sets ====="
output=$(VERIFY=true ${OS_ROOT}/hack/update-generated-clientsets.sh)

if [[ "$?" == "0" ]]; then
  echo "SUCCESS: Generated client sets up to date."
else
  echo  $output
  echo "FAILURE: Generated client sets out of date. Please run hack/update-generated-clientsets.sh"
  exit 1
fi
