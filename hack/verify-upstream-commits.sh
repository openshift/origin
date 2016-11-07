#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

if ! git status &> /dev/null; then
  echo "SKIPPED: Not a Git repository"
  exit 0
fi

os::util::ensure::built_binary_exists 'commitchecker'

echo "===== Verifying UPSTREAM Commits ====="
$commitchecker
echo "SUCCESS: All commits are valid."
