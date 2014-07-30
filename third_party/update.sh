#!/bin/bash

set -e

if ! git diff-index --quiet HEAD -- || test $(git ls-files --exclude-standard --others | wc -l) != 0; then
  echo "You can't have any staged files in git when updating packages."
  echo "Either commit them or unstage them to continue."
  exit 1
fi

THIRD_PARTY_DIR=$(dirname $0)
cd $THIRD_PARTY_DIR

. ./deps.sh

if [ $# -gt 0 ]; then
  PACKAGES="$@"
fi

# Create a temp GOPATH root.  It must be an absolute path
mkdir -p ../output/go_dep_update
cd ../output/go_dep_update
TMP_GO_ROOT=$PWD
cd -
export GOPATH=${TMP_GO_ROOT}

for p in $PACKAGES; do
  echo "Fetching $p"

  # this is the target directory
  mkdir -p src/$p

  extra=
  if [[ "$FROM_GOPATH" != "" ]]; then
    extra=" (fork)"
    mkdir -p "${TMP_GO_ROOT}/src/$p"
    frompath="${FROM_GOPATH}/src/$p/"
    cd "${frompath}"
    if ! git diff-index --quiet HEAD -- || test $(git ls-files --exclude-standard --others | wc -l) != 0; then
      echo "You can't have any staged files in ${frompath} git when updating packages."
      echo "Either commit them or unstage them to continue."
      exit 1
    fi
    cd -
    rsync -a -z -r "${frompath}" "${TMP_GO_ROOT}/src/$p"
  else
    # This will checkout the project into src
    if [ $p == "github.com/GoogleCloudPlatform/kubernetes" ]; then
      git clone https://github.com/GoogleCloudPlatform/kubernetes.git $TMP_GO_ROOT/src/$p
    else
      go get -u -d $p
    fi
  fi

  if [ $p == "github.com/GoogleCloudPlatform/kubernetes" ]; then
    export GOPATH=${TMP_GO_ROOT}/src/github.com/GoogleCloudPlatform/kubernetes/third_party:${TMP_GO_ROOT}
  fi

  # The go get path
  gp=$TMP_GO_ROOT/src/$p

  # Attempt to find the commit hash of the repo
  cd $gp

  HEAD=
  REL_PATH=$(git rev-parse --show-prefix 2>/dev/null)
  if [[ -z "$HEAD" && $REL_PATH != *target/go_dep_update* ]]; then
    # Grab the head if it is git
    HEAD=$(git rev-parse HEAD)
  fi

  # Grab the head if it is mercurial
  if [[ -z "$HEAD" ]] && hg root >/dev/null 2>&1; then
    HEAD=$(hg id -i)
  fi

  cd -

  # Copy the code into the final directory
  rsync -a -z -r --exclude '.git/' --exclude '.hg/' $TMP_GO_ROOT/src/$p/ src/$p

  # Make a nice commit about what everything bumped to
  git add src/$p
  if ! git diff --cached --exit-code > /dev/null 2>&1; then
    git commit -m "bump(${p}): ${HEAD}${extra}"
  fi
done
