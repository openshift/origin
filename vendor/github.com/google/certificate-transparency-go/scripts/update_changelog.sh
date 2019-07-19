#!/bin/bash
#
# Run this script from the top level directory of the ct-go repo e.g.
# with scripts/update_changelog.sh.

# Get and build the correct fork that includes markdown output.
# TODO(Martin2112): replace with upstream repo if/when aktau/github-release#81
# is merged.
go get -u github.com/Martin2112/github-release

# Generate the changelog.
github-release info \
  -r certificate-transparency-go \
  -u google \
  --markdown > CHANGELOG.md

