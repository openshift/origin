#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::util::ensure::built_binary_exists 'godepchecker'
godepchecker "$@"
