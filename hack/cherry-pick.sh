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

if [[ "$#" -ne 1 ]]; then
  echo "You must supply a pull request by number or a Git range in the upstream Kube project" 1>&2
  exit 1
fi
os::build::require_clean_tree # Origin tree must be clean

patch="${TMPDIR}/patch"
kubedir="../../../k8s.io/kubernetes"
if [[ ! -d "${kubedir}" ]]; then
  echo "Expected ${kubedir} to exist" 1>&2
  exit 1
fi

if [[ -z "${NO_REBASE-}" ]]; then
  lastkube="$(go run ${OS_ROOT}/hack/version.go ${OS_ROOT}/Godeps/Godeps.json k8s.io/kubernetes/pkg/api)"
fi

pushd "${kubedir}" > /dev/null
os::build::require_clean_tree
git fetch

selector="$(os::build::commit_range $1 origin/master)"

if [[ -z "${NO_REBASE-}" ]]; then
  echo "++ Generating patch for ${selector} onto ${lastkube} ..." 2>&1
  if git rev-parse kube_rebaser_branch > /dev/null 2>&1; then
    git branch -d kube_rebaser_branch
  fi
  git checkout -b kube_rebaser_branch "${lastkube}"
  git diff -p --raw "${selector}" > "${patch}"
  if ! git apply -3 "${patch}"; then
    git rerere # record pre state
    echo 2>&1
    echo "++ Merge conflicts when generating patch, please resolve conflicts and then press ENTER to continue" 1>&2
    read
  fi
  git rerere # record post state
  # stage any new files
  git add . > /dev/null
  # construct a new patch
  git diff --cached -p --raw --{src,dst}-prefix=a/Godeps/_workspace/src/k8s.io/kubernetes/ > "${patch}"
  # cleanup the current state
  git reset HEAD --hard > /dev/null
  git checkout master > /dev/null
  git branch -d kube_rebaser_branch > /dev/null
else
  echo "++ Generating patch for ${selector} without rebasing ..." 2>&1
  git diff -p --raw --{src,dst}-prefix=a/Godeps/_workspace/src/k8s.io/kubernetes/ "${selector}" > "${patch}"
fi

popd > /dev/null

echo "++ Applying patch ..." 2>&1
echo 2>&1
set +e
git apply --reject "${patch}"
if [[ $? -ne 0 ]]; then
  echo "++ Not all patches applied, merge *.req into your files or rerun with REBASE=1"
  exit 1
fi

set -o errexit
git add .
git commit -m "UPSTREAM: $1: " > /dev/null
git commit --amend
echo 2>&1
echo "++ Done" 2>&1
