#!/bin/bash

set -e

if ! git diff-index --quiet HEAD -- || test $(git ls-files --exclude-standard --others | wc -l) != 0; then
  echo "You can't have any staged files in git when updating packages."
  echo "Either commit them or unstage them to continue."
  exit 1
fi

echo "This command will update Origin with the latest stable_proposed branch or tag"
echo "in your OpenShift fork of Kubernetes."
echo
echo "This command is destructive and will alter the contents of your GOPATH projects"
echo 
echo "Hit ENTER to continue or CTRL+C to cancel"
read

pushd $GOPATH/src/github.com/GoogleCloudPlatform/kubernetes > /dev/null
if [[ $(git remote -v | grep -c 'openshift/kubernetes.git') -eq 0 ]]; then
  echo "You must have the OpenShift kubernetes repo set as a remote in $(pwd)"
  echo
  echo "  $ git remote add openshift git@github.com:openshift/kubernetes.git"
  echo
fi
echo "Fetching latest ..."
git fetch
popd > /dev/null

echo "Restoring Origin dependencies ..."
make clean
godep restore

pushd $GOPATH/src/github.com/GoogleCloudPlatform/kubernetes > /dev/null
git checkout stable_proposed
echo "Restoring any newer Kubernetes dependencies ..."
make clean
godep restore
popd > /dev/null

make clean

echo "Clearing old versions ..."
git rm -r Godeps

echo "Saving dependencies ..."
if ! godep save ./... ; then
  echo "ERROR: Unable to save new dependencies. If packages are listed as not found, try fetching them via 'go get'"
  exit 1
fi
git add .
echo "SUCCESS: Added all new dependencies, review Godeps/Godeps.json"
