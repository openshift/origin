#!/bin/bash

# This script sets up a go workspace locally and builds all go components.
# You can 'source' this file if you want to set up GOPATH in your local shell.

# gitcommit prints the current Git commit information
function gitcommit() {
  set -o errexit
  set -o nounset
  set -o pipefail

  topdir=$(dirname "$0")/..
  cd "${topdir}"

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
}

if [[ -z "$(which go)" ]]; then
  echo "Can't find 'go' in PATH, please fix and retry." >&2
  echo "See http://golang.org/doc/install for installation instructions." >&2
  exit 1
fi

# Travis continuous build uses a head go release that doesn't report
# a version number, so we skip this check on Travis.  Its unnecessary
# there anyway.
if [[ "${TRAVIS:-}" != "true" ]]; then
  GO_VERSION=($(go version))
  if [[ "${GO_VERSION[2]}" < "go1.2" ]]; then
    echo "Detected go version: ${GO_VERSION[*]}." >&2
    echo "OpenShift requires go version 1.2 or greater." >&2
    echo "Please install Go version 1.2 or later" >&2
    exit 1
  fi
fi

OS_REPO_ROOT=$(dirname "${BASH_SOURCE:-$0}")/..
case "$(uname)" in
  Darwin)
    # Make the path absolute if it is not.
    if [[ "${OS_REPO_ROOT}" != /* ]]; then
      OS_REPO_ROOT=${PWD}/${OS_REPO_ROOT}
    fi
    ;;
  Linux)
    # Resolve symlinks.
    OS_REPO_ROOT=$(readlink -f "${OS_REPO_ROOT}")
    ;;
  *)
    echo "Unsupported operating system: \"$(uname)\"" >&2
    exit 1
esac

OS_TARGET="${OS_REPO_ROOT}/_output/go"
mkdir -p "${OS_TARGET}"

OS_GO_PACKAGE=github.com/openshift/origin
OS_GO_PACKAGE_DIR="${OS_TARGET}/src/${OS_GO_PACKAGE}"

OS_GO_PACKAGE_BASEDIR=$(dirname "${OS_GO_PACKAGE_DIR}")
mkdir -p "${OS_GO_PACKAGE_BASEDIR}"

# Create symlink under _output/go/src.
ln -snf "${OS_REPO_ROOT}" "${OS_GO_PACKAGE_DIR}"

GOPATH="${OS_TARGET}:${OS_REPO_ROOT}/Godeps/_workspace"
export GOPATH

# Unset GOBIN in case it already exists in the current session.
unset GOBIN

OS_BUILD_TAGS=${OS_BUILD_TAGS-}