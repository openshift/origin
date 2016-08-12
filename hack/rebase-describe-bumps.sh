#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

"${OS_ROOT}/hack/build-go.sh" tools/rebasehelpers/godepchecker

# Find binary
godepchecker="$(os::build::find-binary godepchecker)"
$godepchecker "$@"
