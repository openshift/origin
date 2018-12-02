#!/bin/bash

# Prepares a multitenant cluster for running the networkpolicy plugin by
#
#   1) creating NetworkPolicy objects (and Namespace labels) that
#      implement the same isolation/sharing as had been configured in
#      the multitenant cluster via "oc adm pod-network".
#
#   2) re-isolating all projects that had previously been joined or
#      made global (since the networkpolicy plugin requires every
#      project to have a distinct NetID).
#
# See the documentation for more information on how to use this script
# (the section "Migrating from ovs-networkpolicy to ovs-multitenant"
# in the "Configuring the SDN" document in the "Installation and
# Configuration" guide).

set -o errexit
set -o nounset
set -o pipefail

plugin="$(oc get clusternetwork default --template='{{.pluginName}}')"
if [[ "${plugin}" != "redhat/openshift-ovs-multitenant" ]]; then
   echo "Migration script must be run while still running multitenant plugin"
   exit 1
fi 

function create_networkpolicy() {
    out=$(oc create --namespace "$1" -f - 2>&1)
    ret=$?
    if [[ ${ret} != 0 && ! "${out}" =~ "(AlreadyExists)" ]]; then
	echo "${out}" 1>&2
	return ${ret}
    fi
}

function default-deny() {
    create_networkpolicy "$1" <<EOF
kind: NetworkPolicy
apiVersion: networking.k8s.io/v1
metadata:
  name: default-deny
spec:
  podSelector:
EOF
}

function allow-from-self() {
    create_networkpolicy "$1" <<EOF
kind: NetworkPolicy
apiVersion: networking.k8s.io/v1
metadata:
  name: allow-from-self
spec:
  podSelector:
  ingress:
  - from:
    - podSelector: {}
EOF
}

function allow-from-other() {
    create_networkpolicy "$1" <<EOF
kind: NetworkPolicy
apiVersion: networking.k8s.io/v1
metadata:
  name: $2
spec:
  podSelector:
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          pod.network.openshift.io/legacy-netid: "$3"
EOF
}

# Delete orphan NetNamespaces
for name in $(oc get netnamespaces --output=custom-columns='NAME:.metadata.name' --no-headers); do
    if ! oc get namespace "${name}" >& /dev/null; then
	echo "Deleting orphan NetNamespace '${name}':"
	oc get netnamespace "${name}" -o yaml | sed -e 's/^/    /'
	oc delete netnamespace "${name}"
	echo ""
    fi
done

# Find multiply-used NetIDs
last_id=""
declare -A shared_ids
for id in $(oc get netnamespaces --output=custom-columns='NETID:.netid' --sort-by='.netid' --no-headers); do
    if [[ "${id}" == "${last_id}" ]]; then
	shared_ids["${id}"]=1
    fi
    last_id="${id}"
done

# Create policies and labels
declare -a shared_namespaces
for netns in $(oc get netnamespaces --output=jsonpath='{range .items[*]}{.netname}:{.netid} {end}'); do
    name="${netns%:*}"
    id="${netns#*:}"
    echo ""
    echo "NAMESPACE: ${name}"

    if [[ "${id}" == "0" ]]; then
	echo "Namespace is global: adding label legacy-netid=${id}"
	oc label namespace "${name}" "pod.network.openshift.io/legacy-netid=${id}" >/dev/null
	if [[ "${name}" != "default" ]]; then
	    shared_namespaces+=("${name}")
	fi

    else
	# All other Namespaces get isolated, but allow traffic from themselves and global
	# namespaces. We define these as separate policies so the allow-from-global-namespaces
	# policy can be deleted if it is not needed.

	default-deny "${name}"
	allow-from-self "${name}"
	allow-from-other "${name}" allow-from-global-namespaces 0

	if [[ -n "${shared_ids[${id}]:-}" ]]; then
	    echo "Namespace used a shared NetNamespace: adding label legacy-netid=${id}"
	    oc label namespace "${name}" "pod.network.openshift.io/legacy-netid=${id}" >/dev/null
	    allow-from-other "${name}" allow-from-legacy-netid-"${id}" "${id}"
	    shared_namespaces+=("${name}")
	fi
    fi
done

echo ""

# Uniquify VNIDs. (We do this separately at the end because it's the only step that actually
# has an effect under the multitenant plugin. So if something goes wrong before this point,
# then the script will bomb out without having damaged anything.)
if [[ "${#shared_namespaces[@]}" != 0 ]]; then
    echo "Renumbering formerly-shared namespaces: ${shared_namespaces[@]}"
    oc adm pod-network isolate-projects "${shared_namespaces[@]}"
fi
