#!/bin/bash

set -o pipefail

dryRun="no"
lvlist=""
prompt="no"
vg_id=""
ip_prefix=""
fstab_path=""
script_name=$(basename "$0")


usage() {
    echo "Usage: ${script_name} [...]"
    echo ''
    echo 'The options that this script accepts are:'
    echo ''
    echo '--dryrun          shows the commands that would be executed'
    echo '--lvlist          has the list of volumes to delete'
    echo '--prompt          prompts for user input before executing every command'
    echo '--vgid            vgid containing lvs'
    echo '--ip-prefix       IP address prefix'
    echo '--fstab-path      provide the path for the fstab file'
    echo '-h, --help        show this help text'
    echo ''
    echo 'This script is useful for cleaning of bricks when they are not in use'
    echo 'by any Gluster volume and heketi does not have any reference to them.'
    echo 'This is possible when the heketi database has been scrubbed of brick'
    echo 'entries to make the db consistent.'
    echo ''
    echo 'The script must be run on each node/pod of Gluster as it cleans up only'
    echo 'bricks local to the node. The script is run in two phases. In the first'
    echo 'phase(scan), a list is made of unused LVs on the node and files'
    echo 'containing the list are created per VG.'
    echo "\$ ${script_name} scan --ip-prefix=\"IP\" --fstab-path=\"/path/to/fstab\""
    echo ''
    echo 'In the second phase(delete), the script is invoked with the files'
    echo 'generated in the scan phase as input.'
    echo "\$ ${script_name} delete --ip-prefix=\"IP\" --fstab-path=\"/path/to/fstab\" --vgid=VGID --lvlist=to_delete_VGID.txt"
    echo ''
    echo 'ip-prefix argument is required to determine bricks local to the node.'
    echo ''
    echo 'fstab-path argument is required and is usually "/var/lib/heketi/fstab"'
    echo 'in a pod and "/etc/fstab" if Gluster is not containerized.'
    echo ''
    echo 'NOTE: make sure heketi is not running while the script is run.'

    echo 'Before running the script in delete mode, check the files that it generated have valid IDs.'

    echo ''
}


function parse_args () {
    args=$(getopt \
              --options h \
              --long dryrun,lvlist:,vgid:,prompt,ip-prefix:,fstab-path:,help \
              -n "${script_name}" \
              --  "$@")
    eval set -- "$args"
    while true; do
        case "$1" in
            --dryrun) dryRun="yes"; shift ;;
            --prompt) prompt="yes"; shift ;;
            --ip-prefix) ip_prefix=$2; shift 2 ;;
            --fstab-path) fstab_path=$2; shift 2 ;;
            --lvlist)
                case "$2" in
                    "") shift 2 ;;
                     *) lvlist="$2" ; shift 2 ;;
                esac ;;
            --vgid)
                case "$2" in
                    "") shift 2 ;;
                     *) vg_id="$2" ; shift 2 ;;
                esac ;;
            -h|--help) usage ; exit 0 ;;
            --) shift ; break ;;
            *)
                echo "failed to parse cli (error at $1)"
                exit 1
            ;;
        esac
    done
}

precmd() {
    echo "RUN CMD: $*" >&2
    if [[ "$dryRun" = "yes" ]]; then
        return 0
    fi
    if [[ "$prompt" = "yes" ]]; then
        read -r -p 'Continue? [y/N]: ' pval
        if ! echo "$pval" | grep -q -i '^y'; then
            return 0
        fi
    fi
    "$@"
}

function checkLocalNodeHasGlusterdRunning() {
    if ! command -v gluster &> /dev/null
    then
        echo "did not find gluster cli"
        exit 2
    fi
    if ! command -v glusterd &> /dev/null
    then
        echo "did not find glusterd"
        exit 2
    fi
    if ! systemctl --quiet is-active glusterd
    then
        echo "Glusterd is not active"
        exit 2
    fi
}

main() {
    case "$1" in
        scan)
            shift
            scan_bricks "$@"
        ;;
        delete)
            shift
            delete_bricks "$@"
        ;;
        summarize)
            shift
            summarize_bricks "$@"
        ;;
        *)
            usage
            echo "error: must specify either 'scan' 'summarize' or 'delete'" >&2
            exit 1
        ;;
    esac
}

get_ip(){
    if [[ -z "$ip_prefix" ]]; then
        echo "error: ip_prefix is required" >&2
        exit 2
    fi

    matches=$(ip a | grep 'inet\b' | grep -c "$ip_prefix")
    if [[ "$matches" -gt 1 ]]
    then
        echo "error: too many ips matching prefix $ip_prefix found." >&2
        exit 1
    fi
    if [[ "$matches" -eq 0 ]]
    then
        echo "error: no ips matching prefix $ip_prefix found." >&2
        exit 1
    fi
    ip a | grep 'inet\b' | grep "$ip_prefix" | awk '{print $2}' | cut -d"/" -f1
}

scan_bricks() {
    parse_args "$@"

    if [[ -z "$fstab_path" ]]; then
        echo "error: fstab path is required" >&2
        exit 2
    fi

    ip=$(get_ip)
    if [[ -z "$ip" ]]; then
        exit 0
    fi

    allvgs=$(vgs | grep vg_ | awk '{print $1}'  | awk -F '_' '{print $2}')

    for vg in $allvgs
    do
        gluster volume info | grep "Brick[0-9]" | grep "$ip" | grep "$vg" | awk '{print $2}'  | awk -F":" '{print $2}' | awk -F'/' '{print $7}' | awk -F'_' '{print $2}' | sort -u > "gluster_bricks_in_${vg}.txt"
    done

    for vg in $allvgs
    do
        lvs | grep brick_  | grep "$vg" |  awk '{print $1}' | awk  -F '_' '{print $2}' | sort -u > "All_in_${vg}.txt"
    done

    for vg in $allvgs
    do
        sdiff "gluster_bricks_in_${vg}.txt" "All_in_${vg}.txt" | grep \> | awk '{print $2}' > "to_delete_${vg}.txt"
        echo "$0 delete --ip-prefix=${ip_prefix} --fstab-path=${fstab_path} --vgid=${vg} --lvlist=to_delete_${vg}.txt --prompt"
    done

    rm -f gluster_bricks_in_*
    rm -f All_in_*
}

summarize_bricks() {
    parse_args "$@"

    if [[ -z "$fstab_path" ]]; then
        echo "error: fstab path is required" >&2
        exit 2
    fi

    ip=$(get_ip)
    if [[ -z "$ip" ]]; then
        exit 0
    fi

    summary "$ip"
}




summary() {
    echo "Mount count"
    mount | grep -c brick_
    echo "bricks in fstab count"
    grep -c brick_ "$fstab_path"
    echo "lvs count"
    lvs | grep -c brick_
    echo "gluster bricks count"
    gluster volume info | grep "^Brick[0-9]" | grep -c "$1"
}


delete_bricks() {

    parse_args "$@"

    if [[ -z "$vg_id" ]]; then
        echo "error: vgid is required" >&2
        exit 2
    fi

    if [[ -z "$fstab_path" ]]; then
        echo "error: fstab path is required" >&2
        exit 2
    fi

    echo ""
    echo "Checking for glusterd on local node"
    checkLocalNodeHasGlusterdRunning
    echo ""

    ip=$(get_ip)
    if [[ -z "$ip" ]]; then
        exit 0
    fi

    if ! [[ -n $lvlist && -f $lvlist ]]; then
        echo "lv list $lvlist not found" >&2
        exit 1
    fi
    # shellcheck disable=SC2207
    lines=($(<"$lvlist"))

    for brick_id in "${lines[@]}"; do
        [[ -n "$brick_id" ]] || continue
        echo "lv id read is: $brick_id"

        precmd umount "/var/lib/heketi/mounts/vg_${vg_id}/brick_${brick_id}"

        if ! precmd sed -i.save "/brick_${brick_id}/d" "$fstab_path"
        then
            echo "failed to update fstab: $fstab_path"
            exit 2
        fi

        tpname=$(lvs "vg_${vg_id}" | grep "brick_${brick_id}" | grep -E -o "tp_[abcdef0-9]+")
        if [[ -n "$tpname" ]]; then
            brick_num=$(lvs --select pool_lv="${tpname}" --options lv_name --no-headings "vg_${vg_id}" | wc -l)
            if [[ $brick_num -ne 1 ]]; then
                echo "thin pool has incorrect number of LVs: $brick_num, exiting" >&2
                exit 2
            fi
            precmd lvremove -f "vg_${vg_id}/${tpname}"
        fi

        precmd rmdir "/var/lib/heketi/mounts/vg_${vg_id}/brick_${brick_id}"

        echo ""
    done

    summary "$ip"

}


main "$@"
