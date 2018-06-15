#!/bin/bash

# wait_for_url attempts to access a url in order to
# determine if it is available to service requests.
#
# $1 - The URL to check
# $2 - Optional prefix to use when echoing a successful result
# $3 - Optional time to sleep between attempts (Default: 0.2s)
# $4 - Optional number of attempts to make (Default: 10)
# attribution: openshift/origin hack/util.sh
function wait_for_url {
	url=$1
	prefix=${2:-}
	wait=${3:-0.5}
	times=${4:-40}

	set +e
	cmd="chroot /host curl -kfLs ${url}"
	for i in $(seq 1 $times); do
		out=$(${cmd})
		if [ $? -eq 0 ]; then
			set -e
			echo "${prefix}${out}"
			return 0
		fi
		sleep $wait
	done
	echo "ERROR: gave up waiting ${wait} seconds ${times} times for ${url} with command ${cmd}"
  set -e
	return 1
}

INSTALL_HOST=${1:-`hostname`}

echo "Running using hostname ${INSTALL_HOST}"

chroot /host docker run -d --name "origin" \
        --privileged --pid=host --net=host \
        -e KUBECONFIG=/etc/origin/master/admin.kubeconfig \
        -v /:/rootfs:ro -v /var/run:/var/run:rw -v /sys:/sys -v /var/lib/docker:/var/lib/docker:rw \
        -v /etc/origin/:/etc/origin/ -v /var/lib/origin:/var/lib/origin \
        openshift/origin start \
        --master-config /etc/origin/master/master-config.yaml \
        --node-config=/etc/origin/node/node-config.yaml \
        --latest-images=true

echo "Waiting for services to come up..."
wait_for_url "https://${INSTALL_HOST}:8443/api"

CMD="chroot /host docker exec -i origin"
echo "Starting registry services..."

set -x

$CMD oc adm registry --latest-images=true
# we're hacking the service to use a node port to reduce deployment complexity
$CMD oc patch service docker-registry -p \
     '{ "spec": { "type": "NodePort", "selector": {"docker-registry": "default"}, "ports": [ {"nodePort": 5000, "port": 5000, "targetPort": 5000}] }}'

set +x
echo "Starting web UI service..."

# TODO: use master cert from /etc/origin/registry/master.server.cert
# create secret volume
# mounted at /etc/cockpit/ws-certs.d/master.server.cert
# use secret volume in template

set -x
$CMD oc create -f /etc/origin/registry/registry-console-template.yaml
$CMD oc new-app --template registry-console-template \
     -p OPENSHIFT_OAUTH_PROVIDER_URL=https://${INSTALL_HOST}:8443,COCKPIT_KUBE_URL=https://${INSTALL_HOST},REGISTRY_HOST=${INSTALL_HOST}:5000
# we're hacking the service to use a node port to reduce deployment complexity
$CMD oc patch service registry-console -p \
     '{ "spec": { "type": "NodePort", "selector": {"name": "registry-console"}, "ports": [ {"name": "https", "nodePort": 443, "port": 9000, "targetPort": 9090}, {"name": "http", "nodePort": 80, "port": 9000, "targetPort": 9090} ] }}'

set +x
echo "Restarting API server"
set -x
chroot /host docker restart origin

set +x
echo "Web UI hosted at https://${INSTALL_HOST}"
