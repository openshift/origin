#!/bin/bash

if ! which origin-version-change &>/dev/null; then
  echo "The 'origin-version-change' was not found in the PATH."
  echo "To build it, run: ./hack/build-go.sh cmd/origin-version-change"
  echo
  exit 1
fi

IGNORE_FILES={$IGNORE_FILES:-"examples/sample-app/github-webhook-example.json"}

sample_files=$(find {api,examples,docs,images,plugins,test} -name "*.json")
ignore_arr=(${IGNORE_FILES//,/ })

for f in $sample_files; do
  if [[ $ignore_arr =~ $f ]]; then
    echo "-> Skipping '$f'"
  else
    echo "-> Processing '$f' ..."
    origin-version-change -r "$f"
  fi
done
