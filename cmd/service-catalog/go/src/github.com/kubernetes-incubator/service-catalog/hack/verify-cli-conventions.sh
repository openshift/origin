#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "===== Verifying CLI Conventions ====="

# ensure we have the latest compiled binaries
os::util::ensure::built_binary_exists 'clicheck'

if ! output=$(clicheck 2>&1)
then
	echo "FAILURE: CLI is not following one or more required conventions:"
	echo "$output"
	exit 1
else
  echo "SUCCESS: CLI is following all tested conventions."
fi
