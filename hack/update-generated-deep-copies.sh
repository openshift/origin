#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

function result_file_name() {
	local version=$1
	if [ "${version}" == "api" ]; then
		echo "pkg/api/deep_copy_generated.go"
	else
		echo "pkg/api/${version}/deep_copy_generated.go"
	fi
}

function generate_version() {
	local version=$1
	local TMPFILE="/tmp/deep_copy_generated.$(date +%s).go"

	echo "Generating for version ${version}"

	cat >> $TMPFILE <<EOF
package ${version}

// AUTO-GENERATED FUNCTIONS START HERE
EOF

	GOPATH=$(godep path):$GOPATH go run cmd/gendeepcopy/deep_copy.go -v ${version} -f - -o "${version}=" >>  $TMPFILE

	cat >> $TMPFILE <<EOF
// AUTO-GENERATED FUNCTIONS END HERE
EOF

	gofmt -w -s $TMPFILE
	mv $TMPFILE `result_file_name ${version}`
}

VERSIONS="api v1beta3 v1"
# To avoid compile errors, remove the currently existing files.
for ver in $VERSIONS; do
	rm -f `result_file_name ${ver}`
done
for ver in $VERSIONS; do
	generate_version "${ver}"
done
