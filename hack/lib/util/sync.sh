#!/bin/bash
#
# This library holds sync utility functions.

# os::sync::fork is responsible for synchronizing specified fork from UPSTREAM commits
# in our vendor directory
#
# Arguments:
#  - 1 - temporary directory where synchronization will take place, if not passed one will be created for you
#  - 2 - the source repository from which take the upstream cherry-picks
#  - 3 - the branch in source repository
#  - 4 - the directory inside the source repository where changes are applied
#  - 5 - the destination repository (fork) where changes are targeted
#  - 6 - the destination repository branch
# Returns:
#  None
function os::sync::fork() {
    local tmpDir="$1"
    local fromRepo="$2"
    local fromBranch="$3"
    local fromDir="$4"
    local toRepo="$5"
    local toBranch="$6"
    local fromRepoName=$(basename ${fromRepo})
    local toRepoName=$(basename ${toRepo})

    fromRepoLocation="${tmpDir}/${fromRepoName}"
    toRepoLocation="${tmpDir}/${toRepoName}"

    mkdir -p "${fromRepoLocation}"
    mkdir -p "${toRepoLocation}"

    pushd "${fromRepoLocation}" > /dev/null
    # if we already have a git repo, assume it's the right one and get the requested branch
    if ! git rev-parse --quiet --is-inside-work-tree &> /dev/null; then
        git clone --quiet --single-branch --branch="${fromBranch}" "${fromRepo}" "${fromRepoLocation}"
    fi
    git fetch --refmap -- origin refs/heads/"${fromBranch}":refs/remotes/origin/"${fromBranch}"
    git checkout --quiet "${fromBranch}"
    git reset --quiet --hard origin/"${fromBranch}"
    firstFromSHA=$(git rev-list --max-parents=0 HEAD)
    newFromSHA=$(git log --oneline --format='%H' -1)
    popd > /dev/null

    pushd "${toRepoLocation}" > /dev/null
    # if we aren't in a git repo, clone it
    if ! git rev-parse --quiet --is-inside-work-tree &> /dev/null; then
        git clone --quiet --single-branch --branch="${toBranch}" "${toRepo}" "${toRepoLocation}"
    fi
    git fetch --refmap -- origin refs/heads/"${toBranch}":refs/remotes/origin/"${toBranch}"
    git checkout --quiet "${toBranch}"
    git reset --quiet --hard origin/"${toBranch}"
    startingFromSHA="${firstFromSHA}"
    if [ -f "${fromRepoName}.sha" ]; then
        startingFromSHA=$(cat "${fromRepoName}.sha")
    fi
    if [ "${newFromSHA}" == "${startingFromSHA}" ]; then
        popd > /dev/null
        echo "already at level: ${newFromSHA}, nothing to sync"
        exit 0
    fi

    os::log::info ""
    os::log::info "Syncing from ${startingFromSHA}..${newFromSHA}"

    # create a clean branch to start from
    set +o errexit
    git branch --quiet -D "${toBranch}-working" &> /dev/null
    set -o errexit
    git checkout --quiet -b "${toBranch}-working"
    popd > /dev/null

    pushd "${fromRepoLocation}" > /dev/null
    patchDir="${tmpDir}/patches"
    rm -rf "${patchDir}" && mkdir -p "${patchDir}"
    index=0
    for commit in $(git log --format='%H' --no-merges --reverse "${startingFromSHA}..${newFromSHA}" -- "${fromDir}"); do
        git format-patch --quiet --raw --start-number=${index} --relative="${fromDir}" "${commit}^..${commit}" -o "${patchDir}"
        index=$((index+=10))
    done
    # remove all commits that had no entries
    find "${patchDir}" -type f -size 0 -exec rm {} \;
    popd > /dev/null

    pushd "${toRepoLocation}" > /dev/null
    # only patch if we have patches
    if [[ $(ls -A "${patchDir}") ]]; then
        # apply the changes
        if ! git am -3 --ignore-whitespace "${patchDir}"/*.patch; then
          echo 2>&1
          os::log::error "Patches do not apply cleanly, continue with 'git am' in ${toRepoLocation}, patches are available in ${patchDir}" 2>&1
          exit 1
        fi
        os::log::info ""
        os::log::info "All patches applied cleanly upstream" 2>&1
    else
        os::log::info ""
        os::log::info "No patches to apply.  Bumping sync commit."
    fi
    # update the marker file
    echo "${newFromSHA}" > "${fromRepoName}.sha"
    git add "${fromRepoName}.sha" && git commit --quiet -m "sync(${fromRepo}): ${newFromSHA}"
    popd > /dev/null

    os::log::info ""
    os::log::info "Don't forget to push your changes from ${toRepoLocation}:"
    os::log::info "pushd ${toRepoLocation} && git push origin ${toBranch}-working:${toBranch}"
    os::log::info ""
}
readonly -f os::sync::fork
