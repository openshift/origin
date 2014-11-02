#!/bin/bash

# This script provides common script functions for the hacks
# Requires OS_REPO_ROOT to be set

# os::build::gitcommit prints the current Git commit information
function os::build::gitcommit() {
  (
    set -o errexit
    set -o nounset
    set -o pipefail

    cd "${OS_REPO_ROOT}"

    # TODO: when we start making tags, switch to git describe?
    if git_commit=$(git rev-parse --short "HEAD^{commit}" 2>/dev/null); then
      # Check if the tree is dirty.
      if ! dirty_tree=$(git status --porcelain) || [[ -n "${dirty_tree}" ]]; then
        echo "${git_commit}-dirty"
      else
        echo "${git_commit}"
      fi
    else
      echo "(none)"
    fi
    return 0
  )
}

# os::build::kube::gitcommit returns the version of Kubernetes we have
# vendored.
function os::build::kube::gitcommit() {
  (
    # Run this in a subshell to prevent settings/variables from leaking.
    set -o errexit
    set -o nounset
    set -o pipefail

    cd "${OS_REPO_ROOT}"

    go run hack/version.go ./Godeps/Godeps.json github.com/GoogleCloudPlatform/kubernetes/pkg/api
  )
}

# os::build::ldflags calculates the -ldflags argument for building OpenShift
function os::build::ldflags() {
  (
    # Run this in a subshell to prevent settings/variables from leaking.
    set -o errexit
    set -o nounset
    set -o pipefail

    cd "${OS_REPO_ROOT}"

    kube_version="$(os::build::kube::gitcommit)"

    declare -a ldflags=()
    ldflags+=(-X "${OS_GO_PACKAGE}/pkg/version.commitFromGit" "$(os::build::gitcommit)")
    ldflags+=(-X "github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitCommit" "${kube_version}")
    #ldflags+=(-X "github.com/GoogleCloudPlatform/kubernetes/pkg/version.gitVersion" "${kube_version}")

    # The -ldflags parameter takes a single string, so join the output.
    echo "${ldflags[*]-}"
  )
}
