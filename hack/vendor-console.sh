#!/bin/bash
#
#  This script vendors the Web Console source files into bindata.go files that can be built into the openshift binary.
#
#  Accepted environment variables are:
#   - GIT_REF:           specifies which branch / tag of the web console to vendor. If set, then any untracked/uncommitted changes
#                        will cause the script to exit with an error. If not set then the current working state of the web console
#                        directory will be used.
#   - CONSOLE_REPO_PATH: specifies a directory path to look for the web console repo.  If not set it is assumed to be
#                        a sibling to this repository.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

CONSOLE_REPO_PATH=${CONSOLE_REPO_PATH:-$OS_ROOT/../origin-web-console}
if [[ ! -d "$CONSOLE_REPO_PATH" ]]; then
  echo "The console repo at path $CONSOLE_REPO_PATH does not exist."
  echo "Make sure you have cloned the web console repo locally:  git@github.com:openshift/origin-web-console.git"
  echo "Or, you can specify a different path with the CONSOLE_REPO_PATH environment variable."
  exit 1
fi

echo "Making sure go-bindata binary has been built..."
pushd vendor/github.com/jteeuwen/go-bindata > /dev/null
  go install ./...
popd > /dev/null
os::util::ensure::gopath_binary_exists 'go-bindata'

if [[ -z "${GIT_REF:+x}" ]]; then
  echo "No git ref specified, using current state of the repo including any unstaged changes...";
else
  echo "Using git ref ${GIT_REF} ..."
  pushd "${CONSOLE_REPO_PATH}" > /dev/null
    if [[ -n "$(git status --porcelain -uall)" ]]; then
      echo "You have untracked or uncommitted changes in your console repository."
      echo "Since a GIT_REF was specified you must stash or commit your changes and then run this again."
      exit 1
    fi
    git checkout "${GIT_REF}"
    console_commit="$(git rev-parse --short HEAD)"
    echo "Vendoring origin-web-console commit ${console_commit}"
  popd > /dev/null
fi

echo "Building bindata.go files..."
pushd "${OS_ROOT}" > /dev/null
  # Put each component in its own go package for compilation performance
  # Strip off the dist folder from each package to flatten the resulting directory structure
  # Force timestamps to unify, and mode to 493 (0755)
  "$(os::util::find::gopath_binary go-bindata)" -nocompress -nometadata -prefix "${CONSOLE_REPO_PATH}/dist"      -pkg "assets" -o "pkg/assets/bindata.go"      "${CONSOLE_REPO_PATH}/dist/..."
  "$(os::util::find::gopath_binary go-bindata)" -nocompress -nometadata -prefix "${CONSOLE_REPO_PATH}/dist.java" -pkg "java"   -o "pkg/assets/java/bindata.go" "${CONSOLE_REPO_PATH}/dist.java/..."

  if [[ -n "${COMMIT:+x}" ]]; then
    if [[ -n "$(git status --porcelain)" ]]; then
      echo "Creating branch and commit..."
      git checkout -b "vendor_console_${console_commit}"
      git add "pkg/assets/bindata.go"
      git add "pkg/assets/java/bindata.go"
      git commit -m "Bump origin-web-console (${console_commit})"
    else
      echo "Nothing to commit."
    fi
  fi
popd > /dev/null

echo "Done vendoring.  To run the console, run 'make clean build' and restart your origin server."
