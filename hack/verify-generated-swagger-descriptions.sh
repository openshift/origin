#!/bin/bash
#
# This script verifies that generated Swagger self-describing documentation is up to date.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "===== Verifying Generated Swagger Descriptions ====="

VERIFY=true "${OS_ROOT}/hack/update-generated-swagger-descriptions.sh"