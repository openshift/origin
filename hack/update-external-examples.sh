#!/bin/bash

# This script pulls down example files (eg templates) from external repositories
# so they can be included directly in our repository.
# Feeds off a README.md file with well defined syntax that informs this
# script how to pull the file down.
source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

# For now the only external examples are in examples/quickstarts.
QUICKSTARTS_DIR="${OS_ROOT}/examples/quickstarts"
(
  cd "${QUICKSTARTS_DIR}"

  rm -vf *.{json,yaml,yml}

  # Assume the README.md file contains lines with URLs for the raw json/yaml file to be downloaded.
  # Specifically look for a line containing https://raw.githubusercontent.com, then
  # look for the first content in ()s on that line, which will be the actual url of the file,
  # then use curl to pull that file down.
  curl -# $(grep https://raw.githubusercontent.com README.md | sed -E 's/.*\((.*)\) -.*/\1 -O/')
)
