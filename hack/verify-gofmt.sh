#!/bin/bash

# GoFmt apparently is changing @ head...

GO_VERSION=($(go version))
echo "Detected go version: $(go version)"

if [[ ${GO_VERSION[2]} != "go1.2" && ${GO_VERSION[2]} != "go1.3" ]]; then
  echo "Unknown go version, skipping gofmt."
  exit 0
fi

REPO_ROOT="$(cd "$(dirname "$0")/../" && pwd -P)"

files="$(find ${REPO_ROOT} -type f | grep "[.]go$" | grep -v "release/\|_output/\|target/\|Godeps/")"
bad=$(gofmt -s -l ${files})
if [[ -n "${bad}" ]]; then
  echo "$bad"
  exit 1
fi
