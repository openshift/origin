#!/bin/bash

# See HACKING.md for usage
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

repo="${UPSTREAM_REPO:-k8s.io/kubernetes}"
package="${UPSTREAM_PACKAGE:-pkg/api}"
UPSTREAM_REPO_LOCATION="${UPSTREAM_REPO_LOCATION:-../../../${repo}}"
pr="$1"

if [[ "$#" -ne 1 ]]; then
  echo "You must supply a pull request by number or a Git range in the upstream ${repo} project" 1>&2
  exit 1
fi
os::build::require_clean_tree # Origin tree must be clean

patch="${TMPDIR:-/tmp}/patch"
rm -rf "${patch}"
mkdir -p "${patch}"
patch="${patch}/cherry-pick"

if [[ ! -d "${UPSTREAM_REPO_LOCATION}" ]]; then
  echo "Expected ${UPSTREAM_REPO_LOCATION} to exist" 1>&2
  exit 1
fi

if [[ -z "${NO_REBASE-}" ]]; then
  lastrev="$(go run ${OS_ROOT}/tools/godepversion/godepversion.go ${OS_ROOT}/Godeps/Godeps.json ${repo}/${package})"
fi

pushd "${UPSTREAM_REPO_LOCATION}" > /dev/null
os::build::require_clean_tree

remote="${UPSTREAM_REMOTE:-origin}"
git fetch ${remote}

selector="$(os::build::commit_range $pr ${remote}/master)"

if [[ -z "${NO_REBASE-}" ]]; then
  echo "++ Generating patch for ${selector} onto ${lastrev} ..." 2>&1
  if git rev-parse last_upstream_branch > /dev/null 2>&1; then
    git branch -d last_upstream_branch
  fi
  git checkout -b last_upstream_branch "${lastrev}"
  git diff -p --raw --binary "${selector}" > "${patch}"
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
  git diff --cached -p --raw --binary --{src,dst}-prefix=a/vendor/${repo}/ > "${patch}"
  # cleanup the current state
  git reset HEAD --hard > /dev/null
  git checkout master > /dev/null
  git branch -D last_upstream_branch > /dev/null
else
  echo "++ Generating patch for ${selector} without rebasing ..." 2>&1
  git diff -p --raw --binary --{src,dst}-prefix=a/vendor/${repo}/ "${selector}" > "${patch}"
fi

popd > /dev/null

echo "++ Applying patch ..." 2>&1
echo 2>&1
set +e
git apply --reject "${patch}"
if [[ $? -ne 0 ]]; then
  echo "++ Not all patches applied, merge *.rej into your files or rerun with REBASE=1"
  exit 1
fi

commit_message="UPSTREAM: $pr: Cherry-picked"
if [ "$repo" != "k8s.io/kubernetes" ]; then
  commit_message="UPSTREAM: $repo: $pr: Cherry-picked"
fi

set -o errexit
git add .
git commit -m "$commit_message" > /dev/null
git commit --amend
echo 2>&1
echo "++ Done" 2>&1
