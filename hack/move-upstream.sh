#!/bin/bash

# See HACKING.md for usage

set -o errexit
set -o nounset
set -o pipefail

OS_ROOT=$(dirname "${BASH_SOURCE}")/..
source "${OS_ROOT}/hack/common.sh"
source "${OS_ROOT}/hack/util.sh"
os::log::install_errexit

# Go to the top of the tree.
cd "${OS_ROOT}"

repo="${UPSTREAM_REPO:-k8s.io/kubernetes}"
package="${UPSTREAM_PACKAGE:-pkg/api}"

patch="${TMPDIR}/patch"
relativedir="../../../${repo}"
if [[ ! -d "${relativedir}" ]]; then
  echo "Expected ${relativedir} to exist" 1>&2
  exit 1
fi

if [[ -z "${NO_REBASE-}" ]]; then
  lastrev="$(go run ${OS_ROOT}/hack/version.go ${OS_ROOT}/Godeps/Godeps.json ${repo}/${package})"
fi

branch="$(git rev-parse --abbrev-ref HEAD)"
selector="origin/master...${branch}"
if [[ -n "${1-}" ]]; then
  selector="$1"
fi

echo "++ Generating patch for ${selector} onto ${lastrev} ..." 2>&1
git diff -p --raw --relative=Godeps/_workspace/src/${repo}/ "${selector}" -- Godeps/_workspace/src/${repo}/ > "${patch}"

pushd "${relativedir}" > /dev/null
os::build::require_clean_tree

# create a new branch
git checkout -b "${branch}" "${lastrev}"

# apply the changes
if ! git apply --reject "${patch}"; then
  echo 2>&1
  echo "++ Patch does not apply cleanly, possible overlapping UPSTREAM patches?" 2>&1
  exit 1
fi

# generate a new commit, fetch the latest, and attempt a rebase to master
git add .
git commit -m "UPSTREAMED"
git fetch
git rebase origin/master -i

echo 2>&1
echo "++ Done" 2>&1
