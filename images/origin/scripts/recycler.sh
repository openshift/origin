#!/bin/bash

# 'recycle' performs an 'rm -rf' on a volume to scrub it clean before its reuse as a cluster resource.
# this script is intended to be used in a pod that performs the scrub.
# the container in the pod should succeed or fail based on the exit status of this script.

set -o errexit
set -o nounset
set -o pipefail

function recycle(){

    # first and only arg is the directory to scrub
    dir=$1

    if [ -z "$dir" ]; then
        echo "Usage:  scrub_directory some/path/to/scrub"
        return 1
    fi

    if [ test ! -e $dir ]; then
        echo "scrub directory $dir does not exist"
        return 1
    fi

    # remove everything but keep the directory for re-use as a volume
    rm -rfv $dir/*

    if [ -z "$(ls -A $dir)" ]; then
        echo "scrub directory $dir is empty"
        return 0
    else
        echo "scrub directory $dir is not empty"
        return 1
    fi
}

if recycle $1; then
    echo "Scrub OK"
    exit 0
else
    exit 1
fi
