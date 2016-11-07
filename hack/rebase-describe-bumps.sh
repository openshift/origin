#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

$godepchecker "$@"
os::util::ensure::built_binary_exists 'godepchecker'
