#!/bin/bash
#
# This script synchronizes specified fork with the UPSTREAM commits.
#

set -o errexit
set -o nounset
set -o pipefail

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

tmpDir=$(mktemp -d)
forksData="$(dirname "${BASH_SOURCE}")/forks.data"

while IFS=" " read -r fromRepo fromBranch fromDir toRepo toBranch; do
    os::log::info "Syncing ${fromDir}..."
    os::sync::fork "${tmpDir}" "${fromRepo}" "${fromBranch}" "${fromDir}" "${toRepo}" "${toBranch}"
done < "${forksData}"
