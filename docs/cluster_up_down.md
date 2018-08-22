# Local Cluster Management

- [Overview](#overview)
- [Getting Started](#getting-started)
  - [Prerequisities](#prerequisites)
  - [Linux](#linux)
- [Persistent Volumes](#persistent-volumes)
- [Using a Proxy](#using-a-proxy)
- [Installing Metrics](#installing-metrics)
- [Installing Logging Aggregation](#installing-logging-aggregation)
- [Installing the Service Catalog](#installing-the-service-catalog)
- [Administrator Access](#administrator-access)
- [Configuration](#configuration)
- [Etcd Data](#etcd-data)
- [Routing](#routing)
- [Specifying Images to Use](#specifying-images-to-use)
- [Accessing the OpenShift Registry Directly](#accessing-the-openshift-registry-directly)

## Prerequisites

| NOTE |
| ---- |
| This command was released with the 1.3+ version of oc client tools, so you must be using version 1.3+ or newer for this command to work. |

To run an OpenShift cluster locally, you must have a compatible version of Docker installed in your environment.
OpenShift officially supports the following versions of Docker:

| OpenShift Version | Docker Version |
| ----------------- | -------------- |
| 3.9 | 1.13 |
| 3.6-3.7 | 1.12 |
| 1.4-1.5 | 1.12 |
| 1.3 | 1.10 |

## Overview

The `oc cluster up` command starts a local OpenShift all-in-one cluster with a configured registry, router, image streams, and default templates.

The `oc cluster up` command will create a default user and project and, once it completes, will allow you to start using the
command line to create and deploy apps with commands like `oc new-app`, `oc new-build`, and `oc run`. It will also print out
a URL to access the management console for your cluster.

## Getting Started

### Linux

Currently Linux (including a Linux VM running on another platform) is the only supported platform for cluster up.

| WARNING |
| ------- |
| The default Firewalld configuration on Fedora blocks access to ports needed by containers running on an OpenShift cluster. Make sure you grant access to these ports. See step 3 below. |
| Check that `sysctl net.ipv4.ip_forward` is set to 1. |

1. Install Docker with your platform's package manager. Consult the [compatibility table](#prerequisites) to select a supported version of Docker.
2. Configure the Docker daemon with an insecure registry parameter of `172.30.0.0/16`
   - In RHEL and Fedora, edit the `/etc/containers/registries.conf` file and add the following lines:
     ```
     [registries.insecure]
     registries = ['172.30.0.0/16']
     ```
     or edit the `/etc/docker/daemon.json` file and add the following:
     ```json
     {
        "insecure-registries": [
          "172.30.0.0/16"
        ]
     }
     ```

   - After editing the config, reload systemd and restart the Docker daemon.
     ```
     $ sudo systemctl daemon-reload
     $ sudo systemctl restart docker
     ```
3. Ensure that your firewall allows containers access to the OpenShift master API (8443/tcp) and DNS (53/udp) endpoints.
   In RHEL and Fedora, you can create a new firewalld zone to enable this access:
   - Determine the Docker bridge network container subnet:
     ```
     docker network inspect -f "{{range .IPAM.Config }}{{ .Subnet }}{{end}}" bridge
     ```
     You should get a subnet like: ```172.17.0.0/16```

   - Create a new firewalld zone for the subnet and grant it access to the API and DNS ports:
     ```
     firewall-cmd --permanent --new-zone dockerc
     firewall-cmd --permanent --zone dockerc --add-source 172.17.0.0/16
     firewall-cmd --permanent --zone dockerc --add-port 8443/tcp
     firewall-cmd --permanent --zone dockerc --add-port 53/udp
     firewall-cmd --permanent --zone dockerc --add-port 8053/udp
     firewall-cmd --reload
     ```

4. Download the Linux `oc` binary from
   [openshift-origin-client-tools-VERSION-linux-64bit.tar.gz](https://github.com/openshift/origin/releases)
   and place it in your path.

   > Please be aware that the 'oc cluster' set of commands are only available in the 1.3+ or newer releases.


5. Open a terminal with a user that has permission to run Docker commands and run:
   ```
   $ oc cluster up
   ```

If you are running `oc cluster up` on a virtual machine in Amazon AWS EC2, you should pass the public hostname and IP address to ensure that the cluster is reachable from your local host. You can retrieve this information from the [internal meta-data endpoints](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-metadata.html#instancedata-data-retrieval):
```
$ metadata_endpoint="http://169.254.169.254/latest/meta-data"
$ public_hostname="$( curl "${metadata_endpoint}/public-hostname" )"
$ public_ip="$( curl "${metadata_endpoint}/public-ipv4" )"
$ oc cluster up --public-hostname="${public_hostname}" --routing-suffix="${public_ip}.nip.io"
```

To stop your cluster, run:
```
$ oc cluster down
```

## Persistent Volumes

(Available starting origin release 1.5.0-alpha.2)
Cluster up creates a set of persistent volumes by default. It exposes a new
flag that allows setting the directory on the host for these volumes (`--host-pv-dir`). The registry and templates for
databases and jenkins will now default to persistent storage.

## Using a Proxy

**Cluster up flags**

> Available starting origin release 1.5.0-alpha.2

Cluster up supports the following flags to specify a proxy:
- `--http-proxy` set to URL of HTTP proxy to use
- `--https-proxy` set to URL of HTTPS proxy to use
- `--no-proxy` specify a comma-separated list of hosts to not proxy. Cluster up automatically adds a set of hosts to this list such
   as the current hostname and the docker registry IP

**Docker Settings**

Your Docker settings should mirror the cluster up settings for HTTP and HTTPS proxy. And if you do specify either of these,
You should at least add the registry service IP (172.30.1.1) to the Docker daemon's NO_PROXY environment variable.
How these settings are specified will vary depending on your platform. Both Docker for Windows and Docker for Mac provide a
settings GUI for specifying a proxy. On RHEL or Fedora, you can add HTTP_PROXY, HTTPS_PROXY, and NO_PROXY variables to
/etc/sysconfig/docker (or /etc/sysconfig/docker-latest depending on which service you're using).
Cluster up will warn you if there is a discrepancy between the Docker settings and the arguments you specify for cluster up.


| WARNING |
| ------- |
| On Docker for Windows, if you specify proxy settings for Docker in its GUI, Docker will apply those settings to every container that is run on the daemon. This requires that you specifically exclude any services that could be accessed container to container (172.30.0.0/16). You can do this by going to Docker Settings -> Proxies and entering the subnet you want to exclude in the 'Bypass' text box. |


## Installing Metrics

You can install metrics components by specifying the `--metrics` argument when invoking `oc cluster up`.

To see metrics in the web console, you must first browse to the Hawkular metrics UI URL displayed when `cluster up` starts.


## Installing Logging Aggregation

| NOTE |
| ---- |
| This feature requires an oc command v1.4 or newer |

You can install logging aggregation components by specifying the `--logging` argument when invoking `oc cluster up`.

With logging aggregation installed, a new link will appear in the logs tab of a running pod in the web console.


## Installing the Service Catalog

| NOTE |
| ---- |
| This feature requires an oc command v3.6 or newer. |
| Enabling this feature renders the entire cluster "Tech Preview". |


You can enable the service catalog component by specifying the `--service-catalog` argument when invoking `oc cluster up`.

Enabling the service catalog has the following effect:

1. The API aggregator is enabled to provide a unified API endpoint for both OpenShift/Kubernetes resources and the new APIs introduced by the service catalog.
1. A service catalog deployment is created to deploy and run the service catalog.
1. The template broker is enabled in the OpenShift master server and registered with the service catalog.
1. The web console is configured to use the new service catalog landing page.

On completion, `oc cluster up` will output a command that you may run if you want to use the template broker with the service catalog. CAUTION: running this command has significant adverse security effects as it enables unauthenticated access to the template broker. This allows anyone who can access your master to provision templates into any project as any user.

The service catalog can be used without the template broker, however no other brokers are provided out of the box with `oc cluster up` (though they can be registered with the service catalog manually).



## Administrator Access

To login as administrator to your cluster, login as `system:admin`:
```
oc login -u system:admin
```
Cluster administration commands are available under `oc adm`

To return to the regular `developer` user, login as that user:
```
oc login -u developer
```

## Configuration

`oc cluster up` creates its configuration by default in `/var/lib/origin/openshift.local.config` on the Docker host.
To specify a different location for it, use the `--host-config-dir` argument. The host directory will be mounted
in the `origin` container at `/var/lib/origin/openshift.local.config`.

A new configuration will be generated by default each time the cluster is started. To make changes to the configuration and
preserve those changes, use the `--use-existing-config` argument when starting your cluster.

If your client is not the Docker host, you can make a local copy of the configuration with Docker cp:

```
docker cp origin:/var/lib/origin/openshift.local.config .
```

## Etcd Data

To persist data across restarts, specify a valid host directory in the `--host-data-dir` argument when starting your cluster
with `oc cluster up`. As long as the same value is specified every time, the data will be preserved across restarts.

If a host data directory is not specified, the data directory used by OpenShift is discarded when the container is destroyed.

## Routing

The default routing suffix used by `oc cluster up` is CLUSTER_IP.nip.io where CLUSTER_IP is the IP address of your cluster.
To use a different suffix, specify it with `--routing-suffix`.

## Specifying Images to Use

By default `oc cluster up` uses `openshift/origin:[released-version]` as its OpenShift image (where [released-version]
corresponds to the release of the `oc` client) and `openshift-origin-${component}:[released-version]` for
other images created by the OpenShift cluster (registry, router, builders, etc). It is possible to use a different set of
images by specifying the image prefix.

To use images from a different registry or with a different namespace, use the --image argument.  In the following example,
myregistry.example.com/ose/origin:latest, myregistry.example.com/ose/origin-router:latest, etc. will be used for your cluster.
```
oc cluster up --image=myregistry.example.com/ose/origin
```

## Accessing the OpenShift Registry Directly

On cluster startup, cluster up creates an OpenShift registry by default. The registry can be accessed directly
via docker commands by using its service IP address.

> Starting with release v1.5.0-alpha.2, the service IP of the registry is fixed at `172.30.1.1`

To determine the service IP of the registry (if using a client prior to v1.5.0-alpha.2):

1. Login as system:admin
   ```
   oc login -u system:admin
   ```

2. Get the registry service from the default namespace
   ```
   oc get svc docker-registry -n default
   ```

3. Log back in as your regular user
   ```
   oc login -u developer
   ```

To push arbitrary images from your local Docker daemon to this registry, you will need to:

1. Obtain your OpenShift token and store it in a variable
   ```
   OPENSHIFT_TOKEN=$(oc whoami -t)
   ```

2. Login to the registry
   (Substitute the registry service IP for 172.30.1.1 if different on your system)
   ```
   docker login -u developer -p ${OPENSHIFT_TOKEN} 172.30.1.1:5000
   ```

3. Tag an image with your registry and namespace, and push it.

   An image tag must be of the format `REGISTRY_IP:5000/NAMESPACE/name[:TAG]`

   where REGISTRY_IP is the IP of your registry, NAMESPACE is an OpenShift namespace that
   you have access to, and TAG is an optional tag that you want to create for the image.

   After an image is pushed, an ImageStream is automatically created for it in the namespace
   that you used to tag it.

   Following is an example of pulling an nginx image and pushing it to your local namespace

   ```
   docker pull nginx:latest
   docker tag nginx:latest 172.30.1.1:5000/myproject/nginx:latest
   docker push 172.30.1.1:5000/myproject/nginx:latest
   ```

   After the image is pushed to the local registry, an ImageStream named `nginx` will be
   created in the `myproject` namespace.
