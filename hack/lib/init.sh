#!/bin/bash

# This script is meant to be the entrypoint for OpenShift Bash scripts to import all of the support
# libraries at once in order to make Bash script preambles as minimal as possible. This script recur-
# sively `source`s *.sh files in this directory tree. As such, no files should be `source`ed outside
# of this script to ensure that we do not attempt to overwrite read-only variables.

if [[ -z "${OS_ROOT:-}" ]]; then
	echo "[ERROR] In order to import OpenShift Bash libraries, \$OS_ROOT must be set."
	exit 1
fi

library_files=( $( find "${OS_ROOT}/hack/lib" -type f -name '*.sh' -not -path '*/hack/lib/init.sh' ) )
# TODO(skuzmets): Move the contents of the following files into respective library files.
library_files+=( "${OS_ROOT}/hack/cmd_util.sh" )
library_files+=( "${OS_ROOT}/hack/common.sh" )
library_files+=( "${OS_ROOT}/hack/text.sh" )
library_files+=( "${OS_ROOT}/hack/util.sh" )

for library_file in "${library_files[@]}"; do
	source "${library_file}"
done

unset library_files library_file