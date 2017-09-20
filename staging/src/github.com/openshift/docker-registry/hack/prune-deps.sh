#!/bin/bash

set -e

dep init
dep prune

# we shouldn't have modified anything
git diff-index --name-only --diff-filter=M HEAD | xargs -r git checkout -f

# now cleanup what's dangling
git clean -x  -f -d
