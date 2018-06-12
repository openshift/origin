#!/bin/bash

# Undoes the effects of the migrate-network-policy.sh script by
# re-isolating and re-making-global the previously isolated/global
# projects.
#
# This only undoes the changes originally made by the migration script
# (or other changes that were intentionally made to look the same as
# the changes made by the migration script). It does not attempt to
# convert arbitrary NetworkPolicy objects into multitenant-style
# isolation.

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
