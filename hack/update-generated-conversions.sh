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
	
	mv $TMPFILE $2
}

DESTINATION_FILE_REL=${1:-""}
DESTINATION_FILE_ROOT="${OS_ROOT}/${DESTINATION_FILE_REL}/pkg/api"
VERSIONS="v1beta3 v1"
for ver in $VERSIONS; do
	mkdir -p "${DESTINATION_FILE_ROOT}/${ver}" || echo $? > /dev/null
	DESTINATION_FILE="${DESTINATION_FILE_ROOT}/${ver}/conversion_generated.go"
	generate_version "${ver}" "${DESTINATION_FILE}"
done
