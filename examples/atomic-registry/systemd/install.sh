#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

installhost="${1:-"$(hostname)"}"

# Defaults
REGISTRYPORT="${REGISTRYPORT:-5000}"
MASTERPORT="${MASTERPORT:-8443}"
CONSOLEPORT="${CONSOLEPORT:-9090}"
REGISTRYIMAGE="${REGISTRYIMAGE:-openshift/origin-docker-registry}"
MASTERIMAGE="${MASTERIMAGE:-openshift/origin}"
CONSOLEIMAGE="${CONSOLEIMAGE:-cockpit/kubernetes}"
REGISTRYTAG="${REGISTRYTAG:-latest}"
MASTERTAG="${MASTERTAG:-latest}"
CONSOLETAG="${CONSOLETAG:-latest}"

echo "Installing using hostname ${installhost}"

function write_config() {
  openshift start master --write-config=/etc/atomic-registry/master \
    --etcd-dir=/var/lib/atomic-registry/etcd \
    --public-master="${installhost}:${MASTERPORT}" \
    --master="https://localhost:${MASTERPORT}" \
    --listen="https://0.0.0.0:${MASTERPORT}" \
    --cors-allowed-origins="${installhost}:${CONSOLEPORT}"
}

function copy_files_to_host() {
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
}

function customize_config() {
  echo "Update custom ports, images and tags"

  echo "REGISTRY_HTTP_SECRET=$(head -c 64 /dev/urandom | base64 -w0)" >> /host/etc/sysconfig/atomic-registry

  sed -i "s/8443/${MASTERPORT}/g" /host/etc/sysconfig/atomic-registry

  echo "OPENSHIFT_OAUTH_PROVIDER_URL=https://${installhost}:${MASTERPORT}" >> /host/etc/sysconfig/atomic-registry-console
  echo "REGISTRY_HOST=${installhost}:${REGISTRYPORT}" >> /host/etc/sysconfig/atomic-registry-console
  echo "DOCKER_REGISTRY_SERVICE_PORT=${REGISTRYPORT}" >> /host/etc/sysconfig/atomic-registry
  echo "REGISTRY_HTTP_ADDR=:${REGISTRYPORT}" >> /host/etc/sysconfig/atomic-registry
  echo "REGISTRYPORT=${REGISTRYPORT}" >> /host/etc/sysconfig/atomic-registry
  echo "REGISTRYIMAGE=${REGISTRYIMAGE}" >> /host/etc/sysconfig/atomic-registry
  echo "REGISTRYTAG=${REGISTRYTAG}" >> /host/etc/sysconfig/atomic-registry
  echo "KUBERNETES_SERVICE_HOST=${installhost}" >> /host/etc/sysconfig/atomic-registry
  echo "KUBERNETES_SERVICE_PORT=${MASTERPORT}" >> /host/etc/sysconfig/atomic-registry
  echo "MASTERPORT=${MASTERPORT}" >> /host/etc/sysconfig/atomic-registry-master
  echo "MASTERIMAGE=${MASTERIMAGE}" >> /host/etc/sysconfig/atomic-registry-master
  echo "MASTERTAG=${MASTERTAG}" >> /host/etc/sysconfig/atomic-registry-master
  echo "CONSOLEPORT=${CONSOLEPORT}" >> /host/etc/sysconfig/atomic-registry-console
  echo "CONSOLEIMAGE=${CONSOLEIMAGE}" >> /host/etc/sysconfig/atomic-registry-console
  echo "CONSOLETAG=${CONSOLETAG}" >> /host/etc/sysconfig/atomic-registry-console
  echo "KUBERNETES_SERVICE_HOST=${installhost}" >> /host/etc/sysconfig/atomic-registry-console
  echo "KUBERNETES_SERVICE_PORT=${MASTERPORT}" >> /host/etc/sysconfig/atomic-registry-console

  echo "Updating login template"
  sed -i 's/  templates: null$/  templates:\n    login: site\/registry-login-template.html/' /host/etc/atomic-registry/master/master-config.yaml

  echo "Files updated"
  for file in /host/etc/sysconfig/atomic*; do
    echo $'\t'"${file}:"
    cat "${file}"
    echo
  done
  chroot /host systemctl daemon-reload
}

function print_next_steps() {
  echo "Optionally edit configuration file authentication /etc/atomic-registry/master/master-config.yaml,"
  echo "and/or add certificates to /etc/atomic-registry/master,"
  echo "then enable and start services:"
  echo "   sudo systemctl enable --now atomic-registry-master.service"
  echo "Once all 3 containers are running (docker ps), run the setup script"
  echo "(you can run it again if it is run early and fails)"
  echo "   sudo /var/run/setup-atomic-registry.sh"
}

write_config
copy_files_to_host
customize_config
print_next_steps
