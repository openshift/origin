#!/bin/bash

INSTALL_HOST=${1:-`hostname`}
echo "Installing using hostname ${INSTALL_HOST}"

# write out configuration
openshift start master --write-config /etc/atomic-registry/master \
  --etcd-dir /var/lib/atomic-registry/etcd \
  --public-master ${INSTALL_HOST} \
  --master https://localhost:8443

echo "Copy files to host"

set -x
mkdir -p /etc/atomic-registry/master/site
mkdir -p /etc/atomic-registry/registry
mkdir -p /etc/atomic-registry/serviceaccount
mkdir -p /host/var/lib/atomic-registry/registry

cp /exports/unit_files/* /host/etc/systemd/system/
cp /exports/config/* /host/etc/sysconfig/
cp /exports/oauthclient.yaml /etc/atomic-registry/master/
cp /exports/setup-atomic-registry.sh /host/var/run/
cp /exports/registry-login-template.html /host/etc/atomic-registry/master/site/

chown -R 1001:root /host/var/lib/atomic-registry/registry
chown -R 1001:root /etc/atomic-registry/registry

set +x
echo "Add serviceaccount token and certificate to registry configuration"
ln /etc/atomic-registry/master/ca.crt /etc/atomic-registry/serviceaccount/ca.crt
echo "default" >> /etc/atomic-registry/serviceaccount/namespace
echo "This directory stores the service account token, namespace text file and certificate to enable the registry to connect to the API master." \
    >> /etc/atomic-registry/serviceaccount/README
cat /etc/atomic-registry/master/ca.crt > /etc/atomic-registry/serviceaccount/service-ca.crt
cat /etc/atomic-registry/master/service-signer.crt >> /etc/atomic-registry/serviceaccount/service-ca.crt

echo "This directory stores the docker/distribution registry configuration file. To secure the service add TLS certificates here and reference them as environment variables." \
    >> /etc/atomic-registry/registry/README
echo "This directory stores configuration and certificates for the API master." \
    >> /etc/atomic-registry/master/README

set -x

# add OpenShift API master URL to web console env file
echo "OPENSHIFT_OAUTH_PROVIDER_URL=https://${INSTALL_HOST}:8443" >> /host/etc/sysconfig/atomic-registry-console
echo "REGISTRY_HOST=${INSTALL_HOST}:5000" >> /host/etc/sysconfig/atomic-registry-console
# generate random secret for multi-registry shared storage deployment
echo "REGISTRY_HTTP_SECRET=$(head -c 64 /dev/urandom | base64 -w0)" >> /host/etc/sysconfig/atomic-registry
echo "DOCKER_REGISTRY_SERVICE_HOST=${INSTALL_HOST}" >> /host/etc/sysconfig/atomic-registry

# load updated systemd unit files
chroot /host systemctl daemon-reload

set +x

echo "Updating login template"
sed -i 's/  templates: null$/  templates:\n    login: site\/registry-login-template.html/' /etc/atomic-registry/master/master-config.yaml

echo "Optionally edit configuration file authentication /etc/atomic-registry/master/master-config.yaml,"
echo "and/or add certificates to /etc/atomic-registry/master,"
echo "then enable and start services:"
echo "   sudo systemctl enable --now atomic-registry-master.service"
echo "Once all 3 containers are running (docker ps), run the setup script"
echo "(you can run it again if it is run early and fails)"
echo "   sudo /var/run/setup-atomic-registry.sh ${INSTALL_HOST}"
