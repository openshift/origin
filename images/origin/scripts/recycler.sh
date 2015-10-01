#!/bin/bash

# 'recycler' performs an 'rm -rf' on a volume to scrub it clean before it's
# reused as a cluster resource. This script is intended to be used in a pod that
# performs the scrub. The container in the pod should succeed or fail based on
# the exit status of this script.

set -e -o pipefail

shopt -s dotglob nullglob

if [[ $# -ne 1 ]]; then
    echo >&2 "Usage: $0 some/path/to/scrub"
    exit 1
fi

# first and only arg is the directory to scrub
dir=$1

if [[ ! -d ${dir} ]]; then
    echo >&2 "Error: scrub directory '${dir}' does not exist"
    exit 1
fi

# shred all files
find ${dir} -type f -exec shred -fuvz {} \;

# remove everything that was left, keeping the directory for re-use as a volume
if rm -rfv ${dir}/*; then
    echo 'Scrub OK'
    exit 0
fi

echo 'Scrub failed'
exit 1
