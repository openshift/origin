#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::util::ensure::built_binary_exists 'origin-version-change'
IGNORE_FILES={$IGNORE_FILES:-"examples/sample-app/github-webhook-example.json"}

sample_files=$(find {api,examples,docs,images,plugins,test} -name "*.json" -o -name "*.yaml")
ignore_arr=(${IGNORE_FILES//,/ })

for f in $sample_files; do
  if [[ $ignore_arr =~ $f ]]; then
    echo "-> Skipping '$f'"
  else
    echo "-> Processing '$f' ..."
    origin-version-change -r "$f"
  fi
done
