#!/bin/bash
#
# This script verifies that generated Swagger self-describing documentation is up to date.

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..

echo "===== Verifying Generated Swagger Descriptions ====="

VERIFY=true "${OS_ROOT}/hack/update-generated-swagger-descriptions.sh"