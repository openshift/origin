#!/bin/bash

set -o errexit
set -o pipefail
set -o nounset

# This allows for the clearing of route statuses, routers don't clear the routes status so some may be stale.
# Upon deletion of the routes status active routers will immediately update with a vaild status

#clears status of all routers
function clear_status() {
    local namespace="${1}"
    local route_name="${2}"
    local my_json_blob; my_json_blob=$(oc get --raw http://localhost:8001/oapi/v1/namespaces/${namespace}/routes/${route_name}/)
    local modified_json; modified_json=$(echo "${my_json_blob}" | jq 'del(.status.ingress)')
    curl -s -X PUT http://localhost:8001/oapi/v1/namespaces/"${namespace}"/routes/"${route_name}"/status --data-binary "${modified_json}" -H "Content-Type: application/json" > /dev/null
    echo "route status for route "${route_name}" in namespace "${namespace}" cleared"
}

#sets up clearing a status set by a specific router
function clear_status_set_by() {
    local router_name="${1}"

    for namespace in $( oc get namespaces -o 'jsonpath={.items[*].metadata.name}' ); do
        local routes; routes=($(oc get routes -o jsonpath='{.items[*].metadata.name}' --namespace="${namespace}" 2>/dev/null))
        if [[ "${#routes[@]}" -ne 0  ]]; then
            for route in "${routes[@]}"; do
                clear_routers_status "${namespace}" "${route}" "${router_name}"
            done
        else
            echo "No routes found for namespace "${namespace}""
        fi
    done

}

# clears the status field of a specific router name
function clear_routers_status() {
    local namespace="${1}"
    local route_name="${2}"
    local router_name="${3}"
    local my_json_blob; my_json_blob=$(oc get --raw http://localhost:8001/oapi/v1/namespaces/"${namespace}"/routes/"${route_name}"/) 
    local modified_json; modified_json=$(echo "${my_json_blob}" | jq '."status"."ingress"|=map(select(.routerName != "'${router_name}'"))')
    if [[ "${modified_json}" != "$(echo "${my_json_blob}" | jq '.')" ]]; then
        curl -s -X PUT http://localhost:8001/oapi/v1/namespaces/"${namespace}"/routes/"${route_name}"/status --data-binary "${modified_json}" -H "Content-Type: application/json" > /dev/null
        echo "route status for route "${route_name}" set by router "${router_name}" cleared"
    else
        echo "route "${route_name}" has no status set by "${router_name}""
    fi
}

function cleanup() {
    if [[ -n "${PROXY_PID:+unset_check}" ]]; then
        kill "${PROXY_PID}"
    fi
}
trap cleanup EXIT

USAGE="Usage:
To clear only the status set by a specific router on all routes in all namespaces
./clear-router-status.sh -r [router_name]

router_name is the name in the deployment config, not the name of the pod. If the router is running it will
immediately update any cleared status.

To clear the status field of a route or all routes in a given namespace
./clear-route-status.sh [namespace] [route-name | ALL]


Example Usage
--------------
To clear the status of all routes in all namespaces:
oc get namespaces | awk '{if (NR!=1) print \$1}' | xargs -n 1 -I %% ./clear-route-status.sh %% ALL

To clear the status of all routes in namespace default:
./clear-route-status.sh default ALL

To clear the status of route example in namespace default:
./clear-route-status.sh default example

NOTE: if a router that admits a route is running it will immediately update the cleared route status 
"

if [[ ${#} -ne 2 || "${@}" == *" help "* ]]; then
    printf "%s" "${USAGE}"
    exit
fi

if ! command -v jq >/dev/null 2>&1; then
    printf "%s\n%s\n" "Command line JSON processor 'jq' not found." "please install 'jq' version greater than 1.4 to use this script."
    exit 1
fi

if ! echo | jq '."status"."ingress"|=map(select(.routerName != "test"))' >/dev/null 2>&1; then
    printf "%s\n%s\n" "Command line JSON processor 'jq' version is incorrect." "Please install 'jq' version greater than 1.4 to use this script"
    exit 1
fi    

oc proxy > /dev/null &
PROXY_PID="${!}"

## attempt to access the proxy until it is online
until curl -s -X GET http://localhost:8001/oapi/v1/ >/dev/null; do
    sleep 1
done

if [[ "${1}" == "-r" ]]; then
    clear_status_set_by "${2}"
    exit
fi

namespace="${1}"
route_name="${2}"

if [[ "${route_name}" == "ALL" ]]; then
    routes=($(oc get routes -o jsonpath='{.items[*].metadata.name}' --namespace="${namespace}" 2>/dev/null))
    if [[ "${#routes[@]}" -ne 0 ]]; then
        for route in "${routes[@]}"; do
            clear_status "${namespace}" "${route}"
        done
    else
        echo "No routes found for namespace "${namespace}""
    fi
else
    clear_status "${namespace}" "${route_name}"
fi

