#!/bin/bash

SERVICES=(atomic-openshift-master)

# enable and start services

for SERVICE in "${SERVICES[@]}"
do
  echo "Starting service ${SERVICE}..."
  chroot /host systemctl enable $SERVICE.service
  chroot /host systemctl start $SERVICE.service
done

# TODO: loop until running...
echo "Waiting for services to come up..."
until curl -kLs https://`hostname`:8443/api
do
  printf "."
  sleep 1
done

CMD="chroot /host docker exec -it origin-master"
# TODO: this needs to be smarter. it will fail on openstack instance with floating IP
IPADDR=`hostname -I | awk '{print $1}'`
echo "Starting registry services..."

set -x

$CMD oadm registry --credentials /etc/origin/master/openshift-registry.kubeconfig
$CMD oc patch service docker-registry -p \
     '{ "spec": { "type": "NodePort", "ports": [ {"nodePort": 5000, "port": 5000, "targetPort": 5000}] }}'

set +x
echo "Starting web UI service..."
HOSTNAME=`hostname`

# TODO: use master cert from /etc/origin/registry/master.server.cert
# create secret volume
# mounted at /etc/cockpit/ws-certs.d/master.server.cert
# use secret volume in template

set -x
$cmd oc create -f /host/etc/origin/registry/registry-ui-template.json
$cmd oc new-app --template cockpit-openshift-template \
     -p OPENSHIFT_OAUTH_PROVIDER_URL=https://${HOSTNAME}:8443,COCKPIT_KUBE_URL=https://${HOSTNAME}
$CMD oc patch service cockpit-kube -p \
     '{ "spec": { "type": "NodePort", "ports": [ {"nodePort": 443, "port": 9000, "targetPort": 9090}] }}'

set +x
echo "Web UI hosted at https://${HOSTNAME}"

