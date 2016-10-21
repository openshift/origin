#!/usr/bin/env bash

set -o pipefail

INSTALL_HOST=$(awk -F ' ' '/^masterPublicURL/ {print $2}' /etc/atomic-registry/master/master-config.yaml | awk '{split($0,a,":"); print a[1] ":" a[2]}')
CONSOLEPORT=$(awk -F '=' '/CONSOLEPORT/ {print $2}' /etc/sysconfig/atomic-registry-console)

# we're running this on the host
# the commands will be exec'd in the master container that has the oc client
CMD="docker exec"

# boostrap the registry components using the supported command
# we'll delete the dc and service components later
$CMD atomic-registry-master oadm registry

# pause for components to create
sleep 3
# we don't need the kubernetes components created during bootstrapping
$CMD atomic-registry-master oc delete dc docker-registry
# Get the service account token for registry to connect to master API
set -x
TOKEN_NAME=$($CMD atomic-registry-master oc get sa registry --template '{{ $secret := index .secrets 0 }} {{ $secret.name }}')
$CMD atomic-registry-master oc get secret ${TOKEN_NAME} --template '{{ .data.token }}' | base64 -d > /etc/atomic-registry/serviceaccount/token

# write registry config to host and reference bindmounted host file
$CMD atomic-registry cat /config.yml > /etc/atomic-registry/registry/config.yml
echo "REGISTRY_CONFIGURATION_PATH=/etc/atomic-registry/registry/config.yml" >> /etc/sysconfig/atomic-registry

# Create oauthclient for web console. required for web console to delegate auth
$CMD atomic-registry-master oc new-app --file=/etc/atomic-registry/master/oauthclient.yaml --param=COCKPIT_KUBE_URL=${INSTALL_HOST}:${CONSOLEPORT}

# restart with these changes
systemctl restart atomic-registry.service
set +x
echo "Launch web console in browser at ${INSTALL_HOST}:${CONSOLEPORT}"
echo "By default, ANY username and ANY password will successfully authenticate."
