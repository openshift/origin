# Atomic Registry managed by systemd

http://docs.projectatomic.io/registry/

## Installation

1. Install

        sudo atomic install projectatomic/atomic-registry-install <hostname>

1. Start system services

        sudo systemctl start atomic-registry-master.service

1. Setup the registry. This script creates the oauth client so the web console can connect. It also configures the registry service account so it can connect to the API master.

        sudo /var/run/setup-atomic-registry.sh <hostname>

1. Until the registry is secured with TLS certificates, configure client docker daemon to **--insecure-registry** and restart.

        /etc/sysconfig/docker
        sudo systemctl restart docker.service

**Optional post-install configuration:**
* configure authentication provider. **NOTE**: by default *ANY* username and password will authenticate users.
* configure storage
  * mount local storage **/var/lib/atomic-registry/registry** or
  * configure cloud storage in **/etc/atomic-registry/registry/config.yml**
* add TLS certificates to services (see below)

## Uninstall

* Uninstall but **retain data** in /var/lib/atomic-registry. This will remove all configuration changes, etc. You can run install steps again and existing data will be available in the new deployment configuration.

        sudo atomic install projectatomic/atomic-registry-install
* Uninstall and **remove data** in /var/lib/atomic-registry. This will remove all images and the datastore. This will completely clean up the environment.

        sudo atomic install projectatomic/atomic-registry-install --remove-data

## Services

| Service and container name | Role | Configuration | Data | Port |
| -------------------------- | ---- | ------------- | ---- | ---- |
| atomic-registry-master | auth, datastore, API | General config, incl auth: /etc/atomic-registry/master/master-config.yaml, Log level: /etc/sysconfig/atomic-registry-master | datastore: /var/lib/atomic-registry/etcd | 8443 |
| atomic-registry | docker registry | /etc/sysconfig/atomic-registry, /etc/atomic-registry/registry/config.yml | images: /var/lib/atomic-registry/registry | 5000 |
| atomic-registry-console | web console | /etc/sysconfig/atomic-registry-console | none (stateless) | 9090 |

## Changing configuration

1. Edit appropriate configuration file(s) on host
1. Restart service via systemd

        sudo systemctl restart <service_name>

## Updating

As a microservice application the three services may theoretically be updated independently. However, it is strongly recommended that the services be updated together to ensure you are deploying a tested configuration.

1. Pull the updated images

        sudo docker pull openshift/origin
        sudo docker pull openshift/origin-docker-registry
        sudo docker pull cockpit/kubernetes
1. Restart the services

        sudo systemctl restart atomic-registry-console
        sudo systemctl restart atomic-registry-master
        sudo systemctl restart atomic-registry

## Data persistence and backup

  The data that should be persisted is the configuration, image data and the registry database. These are mounted on the host. See Service table above for specific paths.

## Secure Registry endpoint

Here we create a self-signed certificate so docker clients can connect using TLS. While other tools like openssl may be used to create certificates, the master API provides a tool that may also be used.

1. `sudo docker exec -it atomic-registry-master bash`
1. `cd /etc/atomic-registry/master`
1. `oadm ca create-server-cert --signer-cert=ca.crt --signer-key=ca.key --signer-serial=ca.serial.txt --hostnames='<hostname(s)>' --cert=registry.crt --key=registry.key`
1. `exit`
1. `sudo cp /etc/atomic-registry/master/registry.* /etc/atomic-registry/registry/`
1. `sudo chown -R 1001:root /etc/atomic-registry/registry/`
1. Edit `/etc/sysconfig/atomic-registry`, uncomment environment variables *REGISTRY_HTTP_TLS_CERTIFICATE* and *REGISTRY_HTTP_TLS_KEY*.
1. `sudo systemctl restart atomic-registry`

### Serving the certificate for docker clients

If you're creating a self-signed certificate key pair you want to make the public CA certificate available to end-users so they don't have to put docker into insecure mode.

1. Edit `/etc/atomic-registry/master/master-config.yaml` and add the following extension.

        assetConfig:
          ...
          extensions:
            - name: certs
              sourceDirectory: /etc/atomic-registry/master/site
1. `sudo cp /etc/atomic-registry/master/ca.crt /etc/atomic-registry/master/site/`
1. `sudo systemctl restart atomic-registry-master`
1. Clients may then save this cert into their docker client and restart the docker daemon

        curl --insecure -O https://<registry_hostname>:8443/console/extensions/certs/ca.crt
        sudo cp ca.crt /etc/docker/certs.d/<registry_hostname>:5000/.
        sudo systemctl restart docker.service
