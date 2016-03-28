#!/bin/bash

SERVICES=(atomic-openshift-master)
for SERVICE in "${SERVICES[@]}"
do
  echo "Installing system service ${SERVICE}..."
  cp /container/etc/systemd/system/${SERVICE}.service /host/etc/systemd/system/${SERVICE}.service
  cp /container/etc/sysconfig/${SERVICE} /host/etc/sysconfig/${SERVICE}
done

chroot /host systemctl daemon-reload

IMAGES=(openshift/origin openshift/origin-docker-registry aweiteka/cockpit-registry:0.95)

for IMAGE in "${IMAGES[@]}"
do
  chroot /host docker pull $IMAGE
done

# write out configuration
openshift start --write-config /etc/origin/ --etcd-dir /var/lib/origin/etcd --volume-dir /var/lib/origin/volumes --public-master `hostname`

echo "Moving node directory to /etc/origin/node"
mv /host/etc/origin/node* /host/etc/origin/node

# Copy install script to host
mkdir -p /host/etc/origin/registry/bin
cp /container/bin/* /host/etc/origin/registry/bin/.
cp /container/etc/origin/registry-ui-template.json /host/etc/origin/registry/registry-ui-template.json

echo "Creating registry UI service certificates..."
cat /etc/origin/master/master.server.crt /etc/origin/master/master.server.key > /etc/origin/registry/master.server.cert

echo "Updating servicesNodePortRange to 443-32767..."
sed -i 's/  servicesNodePortRange:.*$/  servicesNodePortRange: 443-32767/' /etc/origin/master/master-config.yaml

echo "Optionally edit configuration file /etc/origin/master/master-config.yaml,"
echo "add certificates to /etc/origin/master,"
echo "then run 'atomic run atomic-registry-quickstart'"
