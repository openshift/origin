#!/bin/bash

# See HACKING.md for usage
# To apply all the kube UPSTREAM patches to a kubernetes git directory, you can
#  1. Set UPSTREAM_DIR for your kube working directory
#  2. Set TARGET_BRANCH for the new branch to work in
#  3. In your kube git directory, set the current branch to the level to want to apply patches to
#  4. Run `hack/move-upstream.sh master...<commit hash you want to start pulling patches from>`

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

patch="${TMPDIR:-/tmp}/patch"
rm -rf "${patch}"
mkdir -p "${patch}"
relativedir="${UPSTREAM_REPO_LOCATION:-../../../${repo}}"
if [[ ! -d "${relativedir}" ]]; then
  echo "Expected ${relativedir} to exist" 1>&2
  exit 1
fi

if [[ -z "${NO_REBASE-}" ]]; then
  if [[ "${package}" != "." ]]; then
    out="${repo}/${package}"
  else
    out="${repo}"
  fi
  lastrev="$(go run ${OS_ROOT}/tools/godepversion/godepversion.go ${OS_ROOT}/Godeps/Godeps.json ${out})"
fi

branch="${TARGET_BRANCH:-$(git rev-parse --abbrev-ref HEAD)}"
selector="origin/master...${branch}"
if [[ -n "${1-}" ]]; then
  selector="$1"
fi

echo "++ Generating patch for ${selector} onto ${lastrev} ..." 2>&1
index=0
for commit in $(git log --no-merges --format="%H" --reverse "${selector}" -- "Godeps/_workspace/src/${repo}/"); do
  git format-patch --raw --start-number=${index} --relative="Godeps/_workspace/src/${repo}/" "${commit}^..${commit}" -o "${patch}"
  let index+=10
done

# remove all commits that had no entries
find "${patch}" -type f -size 0 -exec rm {} \;

pushd "${relativedir}" > /dev/null
os::build::require_clean_tree

# create a new branch
git checkout -b "${branch}" "${lastrev}"

# apply the changes
if ! git am -3 --ignore-whitespace ${patch}/*.patch; then
  echo 2>&1
  echo "++ Patches do not apply cleanly, continue with 'git am' in ${relativedir}" 2>&1
  exit 1
fi

echo 2>&1
echo "++ All patches applied cleanly upstream" 2>&1
