#!/bin/bash

# 'recycler' performs an 'rm -rf' on a volume to scrub it clean before it's
# reused as a cluster resource. This script is intended to be used in a pod that
# performs the scrub. The container in the pod should succeed or fail based on
# the exit status of this script.

set -o errexit
set -o noglob
set -o nounset
set -o pipefail

if [[ $# -ne 1 ]]; then
    echo >&2 "Usage: $0 some/path/to/scrub"
    exit 1
fi

# first and only arg is the directory to scrub
dir="${1}"

if [[ ! -d "${dir}" ]]; then
    echo >&2 "Error: scrub directory '${dir}' does not exist"
    exit 1
fi

# shred regular files
function recycle_file() {
    filename="${1}"
    uid=$(stat -c "#%u" "${filename}")
    sudo -u "${uid}" shred "${filename}"
}
export -f recycle_file

find "${dir}" -type f -print0 | xargs -r -n 1 -0 bash -c 'recycle_file "$@"' {}

# rm all
function rm_all() {
    filename="${1}"
    uid=$(stat -c "#%u" "${filename}")
    sudo -u "${uid}" rm -rf "${filename}"
}
export -f rm_all

find "${dir}" ! -type d -print0 | xargs -r -n 1 -0 bash -c 'rm_all "$@"' {}

find "${dir}" -mindepth 1 -type d -print0 | sort -zrg | xargs -r -n 1 -0 bash -c 'rm_all "$@"' {}

echo "Scrub OK"
exit 0
