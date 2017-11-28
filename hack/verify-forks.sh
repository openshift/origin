#!/bin/bash
#
# This script verifies forks.data file contents with our vendored directory and
# forks we own.
#

set -o errexit
set -o nounset
set -o pipefail

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

forksData="$(dirname "${BASH_SOURCE}")/forks.data"


declare -A depths=()
depths=(
    ["vendor/cloud.google.com"]="1"
    ["vendor/k8s.io"]="1"
    ["vendor/go4.org"]="0"
    ["vendor/go.pedge.io"]="1"
    ["vendor/google.golang.org"]="1"
    ["vendor/gopkg.in"]="1"
    ["vendor/vbom.ml"]="1"
)

function get_depth() {
    local dir="$1"
    local depth="2"
    for i in "${!depths[@]}"; do
        if [ "${i}" == "${dir}" ] ; then
            depth="${depths[${i}]}"
        fi
    done
    echo "${depth}"
}

ret=0

for dir in $(find vendor/ -mindepth 1 -maxdepth 1 -type d); do
    os::log::info "Checking ${dir}..."
    depth=$(get_depth "${dir}")
    for subdir in $(find "${dir}" -mindepth "${depth}" -maxdepth "${depth}" -type d); do
        # find last bump commit, if it exists
        set +o pipefail
        # TODO: theoretically we should only check verified dependency directory
        # but some of them are transitive dependencies we bump along with k8s
        revRange=$(git log --format='%H' --grep "bump(${subdir#vendor/})" --grep "bump(k8s.io/kubernetes)"  --no-merges -- "${subdir}" | head -n 1)
        if [[ -z ${revRange} ]]; then
            revRange="HEAD"
        else
            revRange="${revRange}..HEAD"
        fi
        if [[ $(git log --no-merges --grep UPSTREAM --reverse "${revRange}" -- "${subdir}") ]] ; then
            if ! grep -q "${subdir}" "${forksData}"; then
                os::log::error "Missing ${subdir} in ${forksData} (${revRange})"
                ret=1
            fi
        fi
    done
done

exit "${ret}"
