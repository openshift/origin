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

echo "Restoring dependencies ..."
godep restore

pushd $GOPATH/src/github.com/GoogleCloudPlatform/kubernetes > /dev/null
git checkout stable_proposed
popd > /dev/null

echo "Saving dependencies ..."
godep save ./...
