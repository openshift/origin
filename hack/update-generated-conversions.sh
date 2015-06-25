#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"

# Go to the top of the tree.
cd "${OS_ROOT}"

os::build::setup_env

function generate_version() {
	local version=$1
	local TMPFILE="/tmp/conversion_generated.$(date +%s).go"

	echo "Generating for version ${version}"

	cat >> $TMPFILE <<EOF
package ${version}

// AUTO-GENERATED FUNCTIONS START HERE
EOF

	go run cmd/genconversion/conversion.go -v ${version} -f - >>  $TMPFILE

	cat >> $TMPFILE <<EOF
// AUTO-GENERATED FUNCTIONS END HERE
EOF

	mv $TMPFILE pkg/api/${version}/conversion_generated.go
}

VERSIONS="v1beta3 v1"
for ver in $VERSIONS; do
	generate_version "${ver}"
done
