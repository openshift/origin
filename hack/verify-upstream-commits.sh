#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

cd $(dirname "${BASH_SOURCE}")/..

ok_upstream_pat='UPSTREAM: (([a-z0-9-_]+: )?[0-9]{4,}:|<carry>:|<drop>:)'
ok_revert_pat='UPSTREAM: revert: [a-f0-9]{7,}: (([a-z0-9-_]+: )?[0-9]{4,}:|<carry>:|<drop>:)'

echo "===== Verifying UPSTREAM Commits ====="

invalid_commits=()

OLDIFS="$IFS"
IFS=$'\n'
while IFS= read -r commit; do
  summary=$(echo $commit | cut -d ' ' -f 2-)
  if [[ $summary =~ 'revert' ]]; then
    if [[ ! $summary =~ $ok_revert_pat ]]; then
      invalid_commits+=($commit)
    fi
  else
    if [[ ! $summary =~ $ok_upstream_pat ]]; then
      invalid_commits+=($commit)
    fi
  fi
done < <(git log --oneline master..HEAD | grep -i upstream)
IFS="$OLDIFS"

if [ "${#invalid_commits[@]}" -gt 0 ]; then
  echo "FAILURE: The following malformed UPSTREAM commits were detected:"
  echo ""
  for commit in "${invalid_commits[@]}"; do
    echo "  $commit"
  done
  echo ""
  echo "UPSTREAM commits must match one of the following patterns:"
  echo ""
  echo "  $ok_upstream_pat"
  echo "  $ok_revert_pat"
  exit 1
fi

echo "SUCCESS: No invalid upstream commits"
