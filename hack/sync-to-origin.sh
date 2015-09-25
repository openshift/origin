#!/bin/bash
#
# Program to sync openshift-sdn changes to origin repository.
#

die() {
    echo "Error: $*" 1>&2
    echo "Old origin sdn changes under '$temp_sync_dir'"
    echo "Note: Re-run of this script will wipe out data in '$temp_sync_dir'"
    exit 1
}

die_with_usage() {
    echo "Error: $*" 1>&2
    echo "$usage" 1>&2
    exit 1
}

validate_args() {
    # Check given origin repo dir is valid
    if [ -z "$origin_repo_dir" ]; then
        die_with_usage "Must specify openshift origin git repo dir"
    fi
    local origin_godeps="$origin_repo_dir/Godeps/Godeps.json"
    if [ ! -f "$origin_godeps" ] || !(grep -q "ImportPath.*github.com/openshift/origin\"" $origin_godeps); then
        die_with_usage "Invalid openshift origin git repo dir: '$origin_repo_dir'"
    fi
    origin_repo_dir=$(readlink -f -- $origin_repo_dir)

    # Check given sdn repo dir is valid
    local sdn_godeps="$sdn_repo_dir/Godeps/Godeps.json"
    if [ ! -f "$sdn_godeps" ] || !(grep -q "ImportPath.*github.com/openshift/openshift-sdn\"" $sdn_godeps); then
        die_with_usage "Invalid openshift-sdn git repo dir: '$sdn_repo_dir'"
    fi
    sdn_repo_dir=$(readlink -f -- $sdn_repo_dir)
}

handle_uncommited_sdn_changes() {
    dir=$1
    if [ -d "$dir" ]; then
        pushd $origin_repo_dir >/dev/null
        if (git diff $dir 2>/dev/null | grep -q '+++ ') || \
           (git diff $dir --cached 2>/dev/null | grep -q '+++ '); then
            echo "Warning: openshift origin repo has uncommited sdn changes under '$dir'"
            echo -n "Continue to override the changes?[y/n]: "
            read input
	    input=$(echo $input | tr '[:upper:]' '[:lower:]')
            if [ "$input" != "y" ] && [ "$input" != "yes" ]; then
                exit 0
            fi
        fi
        popd >/dev/null
    fi
}

copy_files_to_origin() {
    local sdn_pkg_dir="$sdn_repo_dir/pkg"
    local sdn_plugins_dir="$sdn_repo_dir/plugins/osdn"
    local origin_sdn_pkg_dir="$origin_repo_dir/Godeps/_workspace/src/github.com/openshift/openshift-sdn"
    local origin_sdn_plugins_dir="$origin_repo_dir/plugins/osdn"

    if [ ! -d "$sdn_pkg_dir" ] || [ ! -d "$sdn_plugins_dir" ]; then
        die "openshift-sdn repo doesn't contain '$sdn_pkg_dir'|'$sdn_plugins_dir'"
    fi
    if [ ! -d "$origin_sdn_pkg_dir" ] || [ ! -d "$origin_sdn_plugins_dir" ]; then
        echo "Warning: openshift origin repo doesn't contain sdn dirs '$origin_sdn_pkg_dir'|'$origin_sdn_plugins_dir'"
    fi

    # If any uncommited sdn changes found in origin repo, get user input before proceeding further
    handle_uncommited_sdn_changes $origin_sdn_pkg_dir
    handle_uncommited_sdn_changes $origin_sdn_plugins_dir

    # Move old origin sdn changes to temporary sync dir 
    mv -f $origin_sdn_pkg_dir $temp_sync_dir
    mv -f $origin_sdn_plugins_dir $temp_sync_dir

    mkdir -p $origin_sdn_pkg_dir || true
    mkdir -p $origin_sdn_plugins_dir || true

    # Copy new sdn changes to origin repo
    cp -rf $sdn_pkg_dir $origin_sdn_pkg_dir
    cp -rf $sdn_plugins_dir/* $origin_sdn_plugins_dir
}

update_origin_godeps() {
    local origin_godeps="$origin_repo_dir/Godeps/Godeps.json"
    local old_godeps="$temp_sync_dir/oldGodeps.json"
    local new_godeps="$temp_sync_dir/newGodeps.json"
    bump_revision=$(git log | head -n 1 | cut -d ' ' -f 2)

    cp -f $origin_godeps $old_godeps 
    sed "/openshift-sdn/ {
          N
          /Comment/ {
            N
            /Rev/ {
              N
              s/\(openshift-sdn.*\n.*Comment.*\n.*Rev\":\)\(.*\n\)/\1 \"$bump_revision\"\n/
            }
          } 
       }" < $old_godeps > $new_godeps

    if !(grep -q $bump_revision $new_godeps); then
        die "Failed to bump openshift-sdn revision in origin repo"
    fi

    # Update origin godeps
    mv $origin_godeps $temp_sync_dir
    cp -f $new_godeps $origin_godeps
}

cleanup() {
    rm -rf $temp_sync_dir
}

script_dir=$(dirname $(readlink -f -- $0))
sdn_repo_dir=$(dirname $script_dir)
origin_repo_dir=""
bump=false
bump_revision=""
usage="
$(basename "$0") - Program to sync openshift-sdn changes to openshift origin repository.
It will also bump up origin with openshift-sdn latest revision from current branch.
Usage: $0 -r <origin repo dir> [-s <openshift-sdn repo dir>] [-h]
where
    -r  openshift origin top-level git repo dir
    -s  openshift-sdn top-level git repo dir. If not specified, defaults to '$sdn_repo_dir'
    -h  show this help text"
temp_sync_dir="/tmp/sdn-sync-to-orgin"
cleanup
mkdir -p $temp_sync_dir || true

while getopts ':r:s::h' opt; do
  case "$opt" in
    r) origin_repo_dir=$OPTARG
       ;;
    s) sdn_repo_dir=$OPTARG
       ;;
    h) echo "$usage"
       exit 0
       ;;
    :) die_with_usage "Missing argument for -${OPTARG}\n"
       ;;
   \?) die_with_usage "Illegal option: -${OPTARG}\n"
       ;;
  esac
done
shift $((OPTIND - 1))

validate_args
copy_files_to_origin
update_origin_godeps
cleanup

echo "Synced openshift-sdn revision '$bump_revision' to origin repo '$origin_repo_dir'"
