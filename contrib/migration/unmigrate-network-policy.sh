#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

plugin="$(oc get clusternetwork default --template='{{.pluginName}}')"
if [[ "${plugin}" != "redhat/openshift-ovs-multitenant" ]]; then
   echo "Migration script must be run after switching back to multitenant plugin"
   exit 1
fi

declare -A ids
for ns in $(oc get namespaces --output=jsonpath="{range .items[*]}{.metadata.name}:{.metadata.labels['pod\\.network\\.openshift\\.io/legacy-netid']} {end}"); do
    name="${ns%:*}"
    id="${ns#*:}"
    if [[ -n "${id}" ]]; then
	ids["${id}"]+=" ${name}"
    fi
done

for id in ${!ids[@]}; do
    if [[ "${id}" == 0 ]]; then
	echo "Making global:${ids[${id}]}"
	oc adm pod-network make-projects-global ${ids["${id}"]}
    else
	echo "Joining projects:${ids[${id}]}"
	oc adm pod-network join-projects --to ${ids["${id}"]}
    fi
done
