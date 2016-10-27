#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

echo "===== Verifying CLI Conventions ====="

# ensure we have the latest compiled binaries
"${OS_ROOT}/hack/build-go.sh" tools/clicheck

# Find binary
clicheck="$(os::build::find-binary clicheck)"

if [[ -z "$clicheck" ]]; then
  {
    echo "It looks as if you don't have a compiled clicheck binary"
    echo
    echo "If you are running from a clone of the git repo, please run"
    echo "'./hack/build-go.sh tools/clicheck'."
  } >&2
  exit 1
fi

if ! output=`$clicheck 2>&1`
then
	echo "FAILURE: CLI is not following one or more required conventions:"
	echo "$output"
	exit 1
else
  echo "SUCCESS: CLI is following all tested conventions."
fi
