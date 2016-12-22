#!/bin/bash
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

os::build::setup_env

os::util::ensure::built_binary_exists 'gendeepcopy'

gendeepcopy --output-base="${GOPATH}/src" "$@"